package campaign

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/shared/apperr"
)

type JoinLimiter interface {
	Allow(key string, now time.Time) (bool, time.Duration)
}

type JoinCommand struct {
	UserID   string `json:"-"`
	Code     string `json:"code"`
	Password string `json:"password"`
	SheetID  string `json:"sheet_id"`
}

type PreviewInviteCommand struct {
	UserID   string `json:"-"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

type InvitePreview struct {
	Name             string      `json:"name"`
	GMUsername       string      `json:"gm_username"`
	GameSystem       *GameSystem `json:"game_system,omitempty"`
	TemplateID       string      `json:"template_id,omitempty"`
	ScopeName        string      `json:"scope_name"`
	TemplateIDs      []string    `json:"template_ids"`
	PasswordRequired bool        `json:"password_required"`
	PlayerCount      int         `json:"player_count"`
	AlreadyMember    bool        `json:"already_member"`
}

type InviteCommand struct {
	CampaignID string `json:"-"`
	ActorID    string `json:"-"`
}

type PasswordCommand struct {
	CampaignID string `json:"-"`
	ActorID    string `json:"-"`
	Password   string `json:"password"`
}

const (
	campaignJoinKeyPrefix = "campaign:"
	maxJoinPasswordBytes  = 512
)

func campaignJoinKey(campaignID string) string {
	return campaignJoinKeyPrefix + campaignID
}

func (s *ServiceImpl) JoinByCode(ctx context.Context, cmd JoinCommand) (View, error) {
	if err := requireUser(cmd.UserID); err != nil {
		return View{}, err
	}
	if cmd.Code == "" {
		return View{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInviteCodeMissing, nil)
	}
	if len(cmd.Password) > maxJoinPasswordBytes {
		return View{}, apperr.NewError(apperr.CodeInvalidRequest, MsgPasswordInvalid, nil)
	}
	if cmd.SheetID != "" {
		if err := requireSheetID(cmd.SheetID); err != nil {
			return View{}, err
		}
	}
	now := s.now()
	stored, err := s.resolveInvite(ctx, cmd.Code, cmd.Password)
	if err != nil {
		return View{}, err
	}
	if allowed, _ := s.joinLimit.Allow(campaignJoinKey(stored.ID), now); !allowed {
		return View{}, apperr.NewError(apperr.CodeRateLimited, MsgRateLimited, nil)
	}
	if stored.PasswordHash != "" {
		if cmd.Password == "" {
			return View{}, apperr.NewError(apperr.CodePasswordRequired, MsgPasswordRequired, nil)
		}
		if !s.hasher.Verify(stored.PasswordHash, cmd.Password) {
			return View{}, invalidInvite()
		}
	}
	if _, ok := stored.MemberOf(cmd.UserID); ok {
		return s.view(ctx, stored, cmd.UserID), nil
	}
	out, err := s.joinCampaign(ctx, stored, cmd, now)
	if err != nil {
		return View{}, mapJoinError(err)
	}
	PublishMemberJoined(s.publisher, out, cmd.UserID)
	if cmd.SheetID != "" {
		s.publishCampaignEvent(EventSheetLinked, cmd.UserID, out.ID, out.Version, map[string]any{
			"sheet_id": cmd.SheetID,
		})
	}
	return s.view(ctx, &out, cmd.UserID), nil
}

func (s *ServiceImpl) joinCampaign(ctx context.Context, stored *Campaign, cmd JoinCommand, now time.Time) (Campaign, error) {
	return Mutate(ctx, s.tx, s.repository, stored.ID, now, func(txCtx context.Context, c *Campaign) error {
		if c.InviteCode != cmd.Code || c.PasswordHash != stored.PasswordHash {
			return invalidInvite()
		}
		if err := addPlayer(c, cmd.UserID, now); err != nil {
			return err
		}
		if cmd.SheetID == "" {
			return nil
		}
		_, err := s.claimSheet(txCtx, c, cmd.UserID, cmd.SheetID)
		return err
	})
}

func (s *ServiceImpl) PreviewInvite(ctx context.Context, cmd PreviewInviteCommand) (InvitePreview, error) {
	if err := requireUser(cmd.UserID); err != nil {
		return InvitePreview{}, err
	}
	if cmd.Code == "" {
		return InvitePreview{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInviteCodeMissing, nil)
	}
	if len(cmd.Password) > maxJoinPasswordBytes {
		return InvitePreview{}, apperr.NewError(apperr.CodeInvalidRequest, MsgPasswordInvalid, nil)
	}
	now := s.now()
	stored, err := s.resolveInvite(ctx, cmd.Code, cmd.Password)
	if err != nil {
		return InvitePreview{}, err
	}
	if allowed, _ := s.joinLimit.Allow(campaignJoinKey(stored.ID), now); !allowed {
		return InvitePreview{}, apperr.NewError(apperr.CodeRateLimited, MsgRateLimited, nil)
	}
	if stored.PasswordHash != "" {
		if cmd.Password == "" {
			return InvitePreview{}, apperr.NewError(apperr.CodePasswordRequired, MsgPasswordRequired, nil)
		}
		if !s.hasher.Verify(stored.PasswordHash, cmd.Password) {
			return InvitePreview{}, invalidInvite()
		}
	}
	scope := s.scopeOf(ctx, stored)
	ownerID := ""
	for _, member := range stored.Members {
		if member.Owner {
			ownerID = member.UserID
			break
		}
	}
	_, alreadyMember := stored.MemberOf(cmd.UserID)
	return InvitePreview{
		Name:             stored.Name,
		GMUsername:       s.usernameOf(ctx, ownerID),
		GameSystem:       scope.GameSystem,
		TemplateID:       stored.TemplateID,
		ScopeName:        scope.Name,
		TemplateIDs:      templateIDsOf(scope),
		PasswordRequired: stored.PasswordHash != "",
		PlayerCount:      stored.PlayerCount,
		AlreadyMember:    alreadyMember,
	}, nil
}

func (s *ServiceImpl) resolveInvite(ctx context.Context, code, password string) (*Campaign, error) {
	if !validInviteCode(code) {
		s.hasher.Verify(s.hasher.DummyHash(), password)
		return nil, invalidInvite()
	}
	stored, err := s.repository.FetchByInviteCode(ctx, code)
	if errors.Is(err, ErrNotFound) {
		s.hasher.Verify(s.hasher.DummyHash(), password)
		return nil, invalidInvite()
	}
	if err != nil {
		return nil, internalError(MsgCampaignRead, err)
	}
	if !stored.Active() {
		s.hasher.Verify(s.hasher.DummyHash(), password)
		return nil, invalidInvite()
	}
	return stored, nil
}

func mapJoinError(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Code == apperr.CodeCampaignNotFound {
		return invalidInvite()
	}
	return err
}

func (s *ServiceImpl) RotateInviteCode(ctx context.Context, cmd InviteCommand) (View, error) {
	if err := requireUser(cmd.ActorID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	code, err := newInviteCode()
	if err != nil {
		return View{}, internalError(MsgInviteWriteFailed, err)
	}
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(_ context.Context, c *Campaign) error {
		if err := requireGM(c, cmd.ActorID); err != nil {
			return err
		}
		c.InviteCode = code
		return nil
	})
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, &out, cmd.ActorID), nil
}

func (s *ServiceImpl) SetPassword(ctx context.Context, cmd PasswordCommand) (View, error) {
	if err := requireUser(cmd.ActorID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	if !ValidPassword(cmd.Password) {
		return View{}, apperr.NewError(apperr.CodeInvalidRequest, MsgPasswordInvalid, nil)
	}
	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return View{}, internalError(MsgPasswordHashFailed, err)
	}
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(_ context.Context, c *Campaign) error {
		if err := requireGM(c, cmd.ActorID); err != nil {
			return err
		}
		c.PasswordHash = hash
		return nil
	})
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, &out, cmd.ActorID), nil
}

func (s *ServiceImpl) ClearPassword(ctx context.Context, cmd InviteCommand) (View, error) {
	if err := requireUser(cmd.ActorID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(_ context.Context, c *Campaign) error {
		if err := requireGM(c, cmd.ActorID); err != nil {
			return err
		}
		c.PasswordHash = ""
		return nil
	})
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, &out, cmd.ActorID), nil
}

func requireGM(c *Campaign, actorID string) error {
	actor, ok := c.MemberOf(actorID)
	if !ok {
		return notFound()
	}
	if actor.Role != RoleGM {
		return forbidden(MsgGMOnly)
	}
	return nil
}
