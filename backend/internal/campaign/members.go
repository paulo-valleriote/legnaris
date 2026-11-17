package campaign

import (
	"context"
	"slices"

	"legnaris/backend/shared/apperr"
)

const (
	reasonLeft    = "left"
	reasonRemoved = "removed"
)

func memberIndex(c *Campaign, userID string) (int, error) {
	idx := c.indexOf(userID)
	if idx < 0 {
		return -1, notFound()
	}
	return idx, nil
}

func targetIndex(c *Campaign, userID string) (int, error) {
	idx := c.indexOf(userID)
	if idx < 0 {
		return -1, apperr.NewError(apperr.CodeNotFound, MsgMemberNotFound, nil)
	}
	return idx, nil
}

func (s *ServiceImpl) unlink(ctx context.Context, campaignID, userID string) error {
	if err := s.unlinker.UnlinkCharacter(ctx, campaignID, userID); err != nil {
		return internalError(MsgCampaignWrite, err)
	}
	return nil
}

func (s *ServiceImpl) Leave(ctx context.Context, cmd LeaveCommand) error {
	if err := requireUser(cmd.UserID); err != nil {
		return err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return err
	}
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		idx, err := memberIndex(c, cmd.UserID)
		if err != nil {
			return err
		}
		member := c.Members[idx]
		if err := authorize(actionLeave, member, member); err != nil {
			return err
		}
		if err := s.unlink(txCtx, c.ID, cmd.UserID); err != nil {
			return err
		}
		c.Members = slices.Delete(c.Members, idx, idx+1)
		return nil
	})
	if err != nil {
		return err
	}
	s.publishCampaignEvent(EventMemberLeft, cmd.UserID, out.ID, out.Version, map[string]any{
		dataKeyUserID: cmd.UserID,
		"reason":      reasonLeft,
	})
	return nil
}

func (s *ServiceImpl) Promote(ctx context.Context, cmd RoleCommand) (View, error) {
	return s.changeRole(ctx, cmd, actionPromote, RolePlayer, RoleGM, MsgNotAPlayer, s.gmSlotFree)
}

func (s *ServiceImpl) Demote(ctx context.Context, cmd RoleCommand) (View, error) {
	return s.changeRole(ctx, cmd, actionDemote, RoleGM, RolePlayer, MsgNotAGM, s.playerSlotFree)
}

func (s *ServiceImpl) gmSlotFree(c *Campaign) error {
	if c.GMCount >= MaxGMs {
		return full(MsgGMsFull)
	}
	return nil
}

func (s *ServiceImpl) playerSlotFree(c *Campaign) error {
	if c.PlayerCount >= MaxPlayers {
		return full(MsgPlayersFull)
	}
	return nil
}

func (s *ServiceImpl) changeRole(
	ctx context.Context,
	cmd RoleCommand,
	act action,
	from, to Role,
	wrongRoleMessage string,
	slotFree func(*Campaign) error,
) (View, error) {
	if err := requireUser(cmd.ActorID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		actorIdx, err := memberIndex(c, cmd.ActorID)
		if err != nil {
			return err
		}
		targetIdx, err := targetIndex(c, cmd.TargetID)
		if err != nil {
			return err
		}
		if err := authorize(act, c.Members[actorIdx], c.Members[targetIdx]); err != nil {
			return err
		}
		if c.Members[targetIdx].Role != from {
			return apperr.NewError(apperr.CodeInvalidRequest, wrongRoleMessage, nil)
		}
		if err := slotFree(c); err != nil {
			return err
		}
		if to == RoleGM {
			if err := s.unlink(txCtx, c.ID, cmd.TargetID); err != nil {
				return err
			}
		}
		c.Members[targetIdx].Role = to
		return nil
	})
	if err != nil {
		return View{}, err
	}
	s.publishCampaignEvent(EventRoleChanged, cmd.ActorID, out.ID, out.Version, map[string]any{
		dataKeyUserID: cmd.TargetID,
		"from":        string(from),
		"to":          string(to),
	})
	return s.view(ctx, &out, cmd.ActorID), nil
}

func (s *ServiceImpl) Remove(ctx context.Context, cmd RoleCommand) (View, error) {
	if err := requireUser(cmd.ActorID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		actorIdx, err := memberIndex(c, cmd.ActorID)
		if err != nil {
			return err
		}
		targetIdx, err := targetIndex(c, cmd.TargetID)
		if err != nil {
			return err
		}
		if err := authorize(actionRemove, c.Members[actorIdx], c.Members[targetIdx]); err != nil {
			return err
		}
		if err := s.unlink(txCtx, c.ID, cmd.TargetID); err != nil {
			return err
		}
		c.Members = slices.Delete(c.Members, targetIdx, targetIdx+1)
		return nil
	})
	if err != nil {
		return View{}, err
	}
	s.publishCampaignEvent(EventMemberLeft, cmd.ActorID, out.ID, out.Version, map[string]any{
		dataKeyUserID: cmd.TargetID,
		"reason":      reasonRemoved,
	})
	return s.view(ctx, &out, cmd.ActorID), nil
}
