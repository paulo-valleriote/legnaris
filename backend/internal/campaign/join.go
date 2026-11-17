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
	now := s.now()
	stored, err := s.resolveInvite(ctx, cmd)
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
	out, err := Mutate(ctx, s.tx, s.repository, stored.ID, now, func(_ context.Context, c *Campaign) error {
		if c.InviteCode != cmd.Code || c.PasswordHash != stored.PasswordHash {
			return invalidInvite()
		}
		return addPlayer(c, cmd.UserID, now)
	})
	if err != nil {
		return View{}, mapJoinError(err)
	}
	PublishMemberJoined(s.publisher, out, cmd.UserID)
	return s.view(ctx, &out, cmd.UserID), nil
}

func (s *ServiceImpl) resolveInvite(ctx context.Context, cmd JoinCommand) (*Campaign, error) {
	if !validInviteCode(cmd.Code) {
		s.hasher.Verify(s.hasher.DummyHash(), cmd.Password)
		return nil, invalidInvite()
	}
	stored, err := s.repository.FetchByInviteCode(ctx, cmd.Code)
	if errors.Is(err, ErrNotFound) {
		s.hasher.Verify(s.hasher.DummyHash(), cmd.Password)
		return nil, invalidInvite()
	}
	if err != nil {
		return nil, internalError(MsgCampaignRead, err)
	}
	if !stored.Active() {
		s.hasher.Verify(s.hasher.DummyHash(), cmd.Password)
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
