package campaign

import (
	"context"
	"errors"
	"strings"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

type CharacterUnlinker interface {
	UnlinkCharacter(ctx context.Context, campaignID, userID string) error
}

type NameLookup interface {
	Username(ctx context.Context, userID string) (string, error)
}

type CreateGuard interface {
	BeforeCreate(ctx context.Context, ownerID string) error
}

type CreateCommand struct {
	OwnerID    string `json:"-"`
	Name       string `json:"name"`
	TemplateID string `json:"template_id"`
}

type LeaveCommand struct {
	CampaignID string `json:"-"`
	UserID     string `json:"-"`
}

type RoleCommand struct {
	CampaignID string `json:"-"`
	ActorID    string `json:"-"`
	TargetID   string `json:"-"`
}

type MemberView struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     Role      `json:"role"`
	Owner    bool      `json:"owner"`
	JoinedAt time.Time `json:"joined_at"`
}

type View struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	TemplateID  string       `json:"template_id"`
	Status      Status       `json:"status"`
	Version     int64        `json:"version"`
	GMCount     int          `json:"gm_count"`
	PlayerCount int          `json:"player_count"`
	ViewerRole  Role         `json:"viewer_role"`
	ViewerOwner bool         `json:"viewer_owner"`
	Members     []MemberView `json:"members"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Summary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TemplateID  string    `json:"template_id"`
	Version     int64     `json:"version"`
	ViewerRole  Role      `json:"viewer_role"`
	ViewerOwner bool      `json:"viewer_owner"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Service interface {
	Create(ctx context.Context, cmd CreateCommand) (View, error)
	Fetch(ctx context.Context, campaignID, userID string) (View, error)
	ListMine(ctx context.Context, userID string) ([]Summary, error)
	Leave(ctx context.Context, cmd LeaveCommand) error
	Promote(ctx context.Context, cmd RoleCommand) (View, error)
	Demote(ctx context.Context, cmd RoleCommand) (View, error)
	Remove(ctx context.Context, cmd RoleCommand) (View, error)
	Delete(ctx context.Context, cmd DeleteCommand) error
}

const (
	MsgCampaignNotFound     = "campaign not found"
	MsgCampaignIDInvalid    = "campaign id must be a canonical UUID v7"
	MsgCampaignIDMissing    = "campaign id not provided"
	MsgUserRequired         = "authenticated user is required"
	MsgNameInvalid          = "campaign name must have between 1 and 80 characters"
	MsgTemplateInvalid      = "template id is not a valid slug"
	MsgAlreadyMember        = "user is already a member of this campaign"
	MsgPlayersFull          = "campaign has no free player slots"
	MsgGMsFull              = "campaign has no free GM slots"
	MsgForbidden            = "this action is not allowed for your role"
	MsgOwnerOnly            = "only the campaign owner can change roles"
	MsgOwnerCannotLeave     = "the owner cannot leave the campaign"
	MsgOwnerRoleLocked      = "the owner's role cannot be changed"
	MsgOwnerCannotBeRemoved = "the owner cannot be removed"
	MsgUseLeave             = "use the leave action to leave the campaign"
	MsgMemberNotFound       = "member not found in this campaign"
	MsgNotAPlayer           = "target member is not a player"
	MsgNotAGM               = "target member is not a GM"
	MsgCampaignRead         = "failed to read campaign"
	MsgCampaignWrite        = "failed to write campaign"
	MsgConcurrentChange     = "campaign was changed by another request, try again"
	MsgTransactionFailed    = "campaign transaction failed"
	MsgCreateFailed         = "failed to create campaign"
	MsgListFailed           = "failed to list campaigns"
)

type ServiceImpl struct {
	repository Repository
	tx         TransactionRunner
	publisher  events.Publisher
	unlinker   CharacterUnlinker
	names      NameLookup
	guard      CreateGuard
	releaser   OwnershipReleaser
	now        func() time.Time
}

type ServiceOption func(*ServiceImpl)

func WithCharacterUnlinker(u CharacterUnlinker) ServiceOption {
	return func(s *ServiceImpl) { s.unlinker = u }
}

func WithNameLookup(n NameLookup) ServiceOption {
	return func(s *ServiceImpl) { s.names = n }
}

func WithCreateGuard(g CreateGuard) ServiceOption {
	return func(s *ServiceImpl) { s.guard = g }
}

func WithOwnershipReleaser(r OwnershipReleaser) ServiceOption {
	return func(s *ServiceImpl) { s.releaser = r }
}

func WithClock(now func() time.Time) ServiceOption {
	return func(s *ServiceImpl) { s.now = now }
}

func NewService(repository Repository, tx TransactionRunner, publisher events.Publisher, opts ...ServiceOption) Service {
	if publisher == nil {
		publisher = events.NoOp{}
	}
	svc := &ServiceImpl{
		repository: repository,
		tx:         tx,
		publisher:  publisher,
		unlinker:   noUnlinker{},
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

type noUnlinker struct{}

func (noUnlinker) UnlinkCharacter(context.Context, string, string) error { return nil }

func requireUser(userID string) error {
	if userID == "" {
		return apperr.NewError(apperr.CodeUnauthorized, MsgUserRequired, nil)
	}
	return nil
}

func requireCampaignID(id string) error {
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgCampaignIDMissing, nil)
	}
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgCampaignIDInvalid, err)
	}
	return nil
}

func (s *ServiceImpl) Create(ctx context.Context, cmd CreateCommand) (View, error) {
	if err := requireUser(cmd.OwnerID); err != nil {
		return View{}, err
	}
	name := strings.TrimSpace(cmd.Name)
	if !ValidName(name) {
		return View{}, apperr.NewError(apperr.CodeInvalidRequest, MsgNameInvalid, nil)
	}
	if !ValidTemplateID(cmd.TemplateID) {
		return View{}, apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateInvalid, nil)
	}
	id, err := idgen.New()
	if err != nil {
		return View{}, internalError(MsgCreateFailed, err)
	}
	now := s.now()
	created := &Campaign{
		ID:          id,
		Name:        name,
		TemplateID:  cmd.TemplateID,
		Status:      StatusActive,
		Version:     1,
		GMCount:     1,
		PlayerCount: 0,
		Members:     []Member{{UserID: cmd.OwnerID, Role: RoleGM, Owner: true, JoinedAt: now}},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if s.guard != nil {
			if err := s.guard.BeforeCreate(txCtx, cmd.OwnerID); err != nil {
				return err
			}
		}
		return s.repository.Insert(txCtx, created)
	})
	if err != nil {
		return View{}, unwrapTxError(err)
	}
	s.publishCampaignEvent(EventCreated, cmd.OwnerID, id, created.Version, map[string]any{
		"template_id": cmd.TemplateID,
	})
	return s.view(ctx, created, cmd.OwnerID), nil
}

func (s *ServiceImpl) Fetch(ctx context.Context, campaignID, userID string) (View, error) {
	if err := requireUser(userID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(campaignID); err != nil {
		return View{}, err
	}
	stored, err := s.repository.FetchByID(ctx, campaignID)
	if err != nil {
		return View{}, readError(err)
	}
	if _, ok := stored.MemberOf(userID); !ok || !stored.Active() {
		return View{}, notFound()
	}
	return s.view(ctx, stored, userID), nil
}

func (s *ServiceImpl) ListMine(ctx context.Context, userID string) ([]Summary, error) {
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	found, err := s.repository.ListActiveByMember(ctx, userID)
	if err != nil {
		return nil, internalError(MsgListFailed, err)
	}
	summaries := make([]Summary, 0, len(found))
	for i := range found {
		member, _ := found[i].MemberOf(userID)
		summaries = append(summaries, Summary{
			ID:          found[i].ID,
			Name:        found[i].Name,
			TemplateID:  found[i].TemplateID,
			Version:     found[i].Version,
			ViewerRole:  member.Role,
			ViewerOwner: member.Owner,
			UpdatedAt:   found[i].UpdatedAt,
		})
	}
	return summaries, nil
}

func readError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return notFound()
	}
	return internalError(MsgCampaignRead, err)
}

func (s *ServiceImpl) view(ctx context.Context, c *Campaign, viewerID string) View {
	viewer, _ := c.MemberOf(viewerID)
	members := make([]MemberView, 0, len(c.Members))
	for _, m := range c.Members {
		members = append(members, MemberView{
			UserID:   m.UserID,
			Username: s.usernameOf(ctx, m.UserID),
			Role:     m.Role,
			Owner:    m.Owner,
			JoinedAt: m.JoinedAt,
		})
	}
	return View{
		ID:          c.ID,
		Name:        c.Name,
		TemplateID:  c.TemplateID,
		Status:      c.Status,
		Version:     c.Version,
		GMCount:     c.GMCount,
		PlayerCount: c.PlayerCount,
		ViewerRole:  viewer.Role,
		ViewerOwner: viewer.Owner,
		Members:     members,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func (s *ServiceImpl) usernameOf(ctx context.Context, userID string) string {
	if s.names == nil {
		return ""
	}
	name, err := s.names.Username(ctx, userID)
	if err != nil {
		return ""
	}
	return name
}
