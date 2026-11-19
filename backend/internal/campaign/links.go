package campaign

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

const (
	ReasonUnlinked     = "unlinked"
	ReasonSheetDeleted = "sheet_deleted"
	ReasonOwnerLeft    = "owner_left"
)

const dataKeyKind = "kind"

const (
	MsgNpcKindMismatch  = "only npc and enemy sheets can be linked to a campaign"
	MsgLinkGMOnly       = "only game masters can manage linked npc and enemy sheets"
	MsgLinkOwnerOnly    = "only the sheet owner can link or unlink it"
	MsgLinkNotFound     = "sheet link not found"
	MsgLinkRead         = "failed to read sheet links"
	MsgLinkWrite        = "failed to update sheet link"
	MsgLinkStoreMissing = "the NPC link store is not configured"
)

type LinkNpcCommand struct {
	CampaignID string `json:"-"`
	UserID     string `json:"-"`
	SheetID    string `json:"sheet_id"`
}

type UnlinkNpcCommand struct {
	CampaignID string `json:"-"`
	UserID     string `json:"-"`
	SheetID    string `json:"-"`
}

type RevealNpcCommand struct {
	CampaignID string `json:"-"`
	UserID     string `json:"-"`
	SheetID    string `json:"-"`
	Revealed   bool   `json:"-"`
}

type ReadNpcSheetQuery struct {
	CampaignID  string
	ViewerID    string
	SheetID     string
	IfNoneMatch string
}

type NpcLinkView struct {
	ID        string         `json:"id"`
	SheetID   string         `json:"sheet_id"`
	Name      string         `json:"name"`
	Kind      plan.SheetKind `json:"kind"`
	OwnerID   string         `json:"owner_id"`
	OwnerName string         `json:"owner_name"`
	Revealed  bool           `json:"revealed"`
	LinkedAt  time.Time      `json:"linked_at"`
	IsOwner   bool           `json:"is_owner"`
}

type noLinks struct{}

func (noLinks) Upsert(context.Context, *SheetLink) (SheetLink, bool, error) {
	return SheetLink{}, false, apperr.NewError(apperr.CodeInternal, MsgLinkStoreMissing, nil)
}

func (noLinks) Fetch(context.Context, string, string) (*SheetLink, error) {
	return nil, ErrLinkNotFound
}

func (noLinks) SetRevealed(context.Context, string, string, bool) (SheetLink, bool, error) {
	return SheetLink{}, false, ErrLinkNotFound
}

func (noLinks) Delete(context.Context, string, string) (bool, error) {
	return false, nil
}

func (noLinks) ListByCampaignID(context.Context, string) ([]SheetLink, error) {
	return nil, nil
}

func (noLinks) DeleteBySheetID(context.Context, string) ([]SheetLink, error) {
	return nil, nil
}

func (noLinks) DeleteByOwner(context.Context, string, string) ([]SheetLink, error) {
	return nil, nil
}

func (noLinks) DeleteByCampaignIDBatch(context.Context, string, int) (int64, error) {
	return 0, nil
}

func linkableKind(kind plan.SheetKind) bool {
	return kind == plan.KindNPC || kind == plan.KindEnemy
}

func kindMismatch(message string) *apperr.Error {
	return apperr.NewError(apperr.CodeSheetKindMismatch, message, nil)
}

func linkNotFound() *apperr.Error {
	return apperr.NewError(apperr.CodeNotFound, MsgLinkNotFound, nil)
}

func linkWriteError(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	return internalError(MsgLinkWrite, err)
}

func (s *ServiceImpl) LinkNpc(ctx context.Context, cmd LinkNpcCommand) (NpcLinkView, error) {
	if err := requireUser(cmd.UserID); err != nil {
		return NpcLinkView{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return NpcLinkView{}, err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return NpcLinkView{}, err
	}
	var (
		link    SheetLink
		stored  sheet.Sheet
		created bool
	)
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		member, ok := c.MemberOf(cmd.UserID)
		if !ok {
			return notFound()
		}
		if member.Role != RoleGM {
			return forbidden(MsgLinkGMOnly)
		}
		found, err := s.sheets.FetchByID(txCtx, cmd.SheetID)
		if errors.Is(err, sheeterr.ErrSheetNotFound) {
			return sheetNotFound()
		}
		if err != nil {
			return internalError(MsgSheetRead, err)
		}
		if found.OwnerID != cmd.UserID {
			return sheetNotFound()
		}
		if !linkableKind(found.Kind) {
			return kindMismatch(MsgNpcKindMismatch)
		}
		if found.TemplateID != c.TemplateID {
			return apperr.NewError(apperr.CodeTemplateMismatch, MsgTemplateMismatch, nil)
		}
		linkID, err := idgen.New()
		if err != nil {
			return internalError(MsgLinkWrite, err)
		}
		candidate := SheetLink{
			ID:         linkID,
			CampaignID: c.ID,
			SheetID:    found.ID,
			OwnerID:    cmd.UserID,
			Revealed:   false,
			LinkedAt:   s.now(),
		}
		link, created, err = s.links.Upsert(txCtx, &candidate)
		if err != nil {
			return linkWriteError(err)
		}
		stored = *found
		return nil
	})
	if err != nil {
		return NpcLinkView{}, err
	}
	if created {
		s.publishCampaignEvent(EventSheetLinked, cmd.UserID, out.ID, out.Version, map[string]any{
			linkSheetIDField: stored.ID,
			dataKeyKind:      string(stored.Kind),
			linkOwnerIDField: stored.OwnerID,
		})
	}
	return s.npcLinkView(ctx, link, stored.Name, stored.Kind, cmd.UserID), nil
}

func (s *ServiceImpl) UnlinkNpc(ctx context.Context, cmd UnlinkNpcCommand) error {
	if err := requireUser(cmd.UserID); err != nil {
		return err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return err
	}
	var (
		removed bool
		kind    plan.SheetKind
		ownerID string
	)
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		member, ok := c.MemberOf(cmd.UserID)
		if !ok {
			return notFound()
		}
		if member.Role != RoleGM {
			return forbidden(MsgLinkGMOnly)
		}
		found, err := s.sheets.FetchByID(txCtx, cmd.SheetID)
		if errors.Is(err, sheeterr.ErrSheetNotFound) {
			return sheetNotFound()
		}
		if err != nil {
			return internalError(MsgSheetRead, err)
		}
		if found.OwnerID != cmd.UserID {
			return forbidden(MsgLinkOwnerOnly)
		}
		if _, err := s.links.Fetch(txCtx, c.ID, found.ID); errors.Is(err, ErrLinkNotFound) {
			return nil
		} else if err != nil {
			return internalError(MsgLinkRead, err)
		}
		deleted, err := s.links.Delete(txCtx, c.ID, found.ID)
		if err != nil {
			return internalError(MsgLinkWrite, err)
		}
		if !deleted {
			return nil
		}
		removed = true
		kind = found.Kind
		ownerID = found.OwnerID
		return nil
	})
	if err != nil {
		return err
	}
	if removed {
		s.publishCampaignEvent(EventSheetUnlinked, cmd.UserID, out.ID, out.Version, map[string]any{
			linkSheetIDField: cmd.SheetID,
			dataKeyKind:      string(kind),
			linkOwnerIDField: ownerID,
			dataKeyReason:    ReasonUnlinked,
		})
	}
	return nil
}

func (s *ServiceImpl) ListNpcLinks(ctx context.Context, campaignID, viewerID string) ([]NpcLinkView, error) {
	if err := requireUser(viewerID); err != nil {
		return nil, err
	}
	if err := requireCampaignID(campaignID); err != nil {
		return nil, err
	}
	c, err := s.activeCampaignFor(ctx, campaignID, viewerID)
	if err != nil {
		return nil, err
	}
	member, _ := c.MemberOf(viewerID)
	found, err := s.links.ListByCampaignID(ctx, campaignID)
	if err != nil {
		return nil, internalError(MsgLinkRead, err)
	}
	views := make([]NpcLinkView, 0, len(found))
	for _, link := range found {
		if member.Role != RoleGM && !link.Revealed {
			continue
		}
		stored, err := s.sheets.FetchByID(ctx, link.SheetID)
		if errors.Is(err, sheeterr.ErrSheetNotFound) {
			continue
		}
		if err != nil {
			return nil, internalError(MsgSheetRead, err)
		}
		if !linkableKind(stored.Kind) {
			continue
		}
		views = append(views, s.npcLinkView(ctx, link, stored.Name, stored.Kind, viewerID))
	}
	return views, nil
}

type revealOutcome struct {
	link    SheetLink
	sheet   sheet.Sheet
	changed bool
}

func (s *ServiceImpl) SetNpcRevealed(ctx context.Context, cmd RevealNpcCommand) (NpcLinkView, error) {
	if err := requireUser(cmd.UserID); err != nil {
		return NpcLinkView{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return NpcLinkView{}, err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return NpcLinkView{}, err
	}
	var outcome revealOutcome
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		return s.applyNpcReveal(txCtx, c, cmd, &outcome)
	})
	if err != nil {
		return NpcLinkView{}, err
	}
	if outcome.changed {
		eventType := EventSheetHidden
		if cmd.Revealed {
			eventType = EventSheetRevealed
		}
		s.publishCampaignEvent(eventType, cmd.UserID, out.ID, out.Version, map[string]any{
			linkSheetIDField: outcome.sheet.ID,
			dataKeyKind:      string(outcome.sheet.Kind),
			linkOwnerIDField: outcome.sheet.OwnerID,
		})
	}
	return s.npcLinkView(ctx, outcome.link, outcome.sheet.Name, outcome.sheet.Kind, cmd.UserID), nil
}

func (s *ServiceImpl) applyNpcReveal(txCtx context.Context, c *Campaign, cmd RevealNpcCommand, outcome *revealOutcome) error {
	member, ok := c.MemberOf(cmd.UserID)
	if !ok {
		return notFound()
	}
	if member.Role != RoleGM {
		return forbidden(MsgLinkGMOnly)
	}
	found, err := s.sheets.FetchByID(txCtx, cmd.SheetID)
	if errors.Is(err, sheeterr.ErrSheetNotFound) {
		return sheetNotFound()
	}
	if err != nil {
		return internalError(MsgSheetRead, err)
	}
	link, changed, err := s.links.SetRevealed(txCtx, c.ID, cmd.SheetID, cmd.Revealed)
	if errors.Is(err, ErrLinkNotFound) {
		return linkNotFound()
	}
	if err != nil {
		return internalError(MsgLinkWrite, err)
	}
	outcome.link = link
	outcome.sheet = *found
	outcome.changed = changed
	return nil
}

func (s *ServiceImpl) ReadNpcSheet(ctx context.Context, q ReadNpcSheetQuery) (LinkedSheetResult, error) {
	if err := requireUser(q.ViewerID); err != nil {
		return LinkedSheetResult{}, err
	}
	if err := requireCampaignID(q.CampaignID); err != nil {
		return LinkedSheetResult{}, err
	}
	if err := requireSheetID(q.SheetID); err != nil {
		return LinkedSheetResult{}, err
	}
	c, err := s.activeCampaignFor(ctx, q.CampaignID, q.ViewerID)
	if err != nil {
		return LinkedSheetResult{}, err
	}
	member, _ := c.MemberOf(q.ViewerID)
	link, err := s.links.Fetch(ctx, c.ID, q.SheetID)
	if errors.Is(err, ErrLinkNotFound) {
		return LinkedSheetResult{}, linkNotFound()
	}
	if err != nil {
		return LinkedSheetResult{}, internalError(MsgLinkRead, err)
	}
	if member.Role != RoleGM && !link.Revealed {
		return LinkedSheetResult{}, linkNotFound()
	}
	stored, err := s.sheets.FetchByID(ctx, q.SheetID)
	if errors.Is(err, sheeterr.ErrSheetNotFound) {
		return LinkedSheetResult{}, sheetNotFound()
	}
	if err != nil {
		return LinkedSheetResult{}, internalError(MsgSheetRead, err)
	}
	if !linkableKind(stored.Kind) {
		return LinkedSheetResult{}, linkNotFound()
	}
	etag := SheetETag(stored.Version, stored.LayoutVersion, member.Role)
	if ETagMatches(q.IfNoneMatch, etag) {
		return LinkedSheetResult{ETag: etag, NotModified: true}, nil
	}
	return LinkedSheetResult{ETag: etag, Sheet: stored}, nil
}

func (s *ServiceImpl) npcLinkView(ctx context.Context, link SheetLink, name string, kind plan.SheetKind, viewerID string) NpcLinkView {
	return NpcLinkView{
		ID:        link.ID,
		SheetID:   link.SheetID,
		Name:      name,
		Kind:      kind,
		OwnerID:   link.OwnerID,
		OwnerName: s.usernameOf(ctx, link.OwnerID),
		Revealed:  link.Revealed,
		LinkedAt:  link.LinkedAt,
		IsOwner:   link.OwnerID == viewerID,
	}
}

func (s *ServiceImpl) removeOwnerLinks(ctx context.Context, campaignID, ownerID string) ([]SheetLink, error) {
	removed, err := s.links.DeleteByOwner(ctx, campaignID, ownerID)
	if err != nil {
		return nil, internalError(MsgLinkWrite, err)
	}
	return removed, nil
}

func (s *ServiceImpl) publishOwnerLeft(ctx context.Context, actorID string, c Campaign, removed []SheetLink) {
	for _, link := range removed {
		stored, err := s.sheets.FetchByID(ctx, link.SheetID)
		if err != nil {
			continue
		}
		s.publishCampaignEvent(EventSheetUnlinked, actorID, c.ID, c.Version, map[string]any{
			linkSheetIDField: link.SheetID,
			dataKeyKind:      string(stored.Kind),
			linkOwnerIDField: link.OwnerID,
			dataKeyReason:    ReasonOwnerLeft,
		})
	}
}

type SheetLinkCleaner struct {
	links     LinkStore
	publisher events.Publisher
}

func NewSheetLinkCleaner(links LinkStore, publisher events.Publisher) *SheetLinkCleaner {
	if publisher == nil {
		publisher = events.NoOp{}
	}
	return &SheetLinkCleaner{links: links, publisher: publisher}
}

func (c *SheetLinkCleaner) DeleteForSheet(ctx context.Context, sheetID string) ([]sheet.RemovedLink, error) {
	removed, err := c.links.DeleteBySheetID(ctx, sheetID)
	if err != nil {
		return nil, err
	}
	out := make([]sheet.RemovedLink, 0, len(removed))
	for _, link := range removed {
		out = append(out, sheet.RemovedLink{
			CampaignID: link.CampaignID,
			SheetID:    link.SheetID,
			OwnerID:    link.OwnerID,
		})
	}
	return out, nil
}

func (c *SheetLinkCleaner) PublishSheetUnlinked(kind plan.SheetKind, removed []sheet.RemovedLink) {
	for _, link := range removed {
		actor := link.OwnerID
		c.publisher.Publish(events.Event{
			Type:          EventSheetUnlinked,
			ActorID:       &actor,
			AggregateType: aggregateType,
			AggregateID:   link.CampaignID,
			Data: map[string]any{
				linkSheetIDField: link.SheetID,
				dataKeyKind:      string(kind),
				linkOwnerIDField: link.OwnerID,
				dataKeyReason:    ReasonSheetDeleted,
			},
		})
	}
}
