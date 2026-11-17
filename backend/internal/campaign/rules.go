package campaign

import "legnaris/backend/shared/apperr"

type action int

const (
	actionLeave action = iota + 1
	actionPromote
	actionDemote
	actionRemove
)

func authorize(act action, actor Member, target Member) *apperr.Error {
	switch act {
	case actionLeave:
		if actor.Owner {
			return forbidden(MsgOwnerCannotLeave)
		}
		return nil
	case actionPromote, actionDemote:
		return authorizeRoleChange(actor, target)
	case actionRemove:
		return authorizeRemove(actor, target)
	}
	return forbidden(MsgForbidden)
}

func authorizeRoleChange(actor Member, target Member) *apperr.Error {
	if !actor.Owner {
		return forbidden(MsgOwnerOnly)
	}
	if target.Owner {
		return forbidden(MsgOwnerRoleLocked)
	}
	return nil
}

func authorizeRemove(actor Member, target Member) *apperr.Error {
	if actor.UserID == target.UserID {
		return forbidden(MsgUseLeave)
	}
	if target.Owner {
		return forbidden(MsgOwnerCannotBeRemoved)
	}
	if actor.Owner {
		return nil
	}
	if actor.Role == RoleGM && target.Role == RolePlayer {
		return nil
	}
	return forbidden(MsgForbidden)
}

func forbidden(message string) *apperr.Error {
	return apperr.NewError(apperr.CodeCampaignForbidden, message, nil)
}
