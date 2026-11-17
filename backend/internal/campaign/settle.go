package campaign

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/apperr"
)

const (
	dataKeyFromUserID = "from_user_id"
	dataKeyToUserID   = "to_user_id"
)

type OwnershipPort interface {
	ReserveOwnership(ctx context.Context, userID string) error
	ReleaseOwnership(ctx context.Context, userID string) error
}

type AccountSettler struct {
	repo      Repository
	tx        TransactionRunner
	ownership OwnershipPort
	unlinker  CharacterUnlinker
	publisher events.Publisher
	now       func() time.Time
}

func NewAccountSettler(repo Repository, tx TransactionRunner, ownership OwnershipPort, unlinker CharacterUnlinker, publisher events.Publisher, now func() time.Time) (*AccountSettler, error) {
	if repo == nil || tx == nil || ownership == nil || unlinker == nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgAccountSettlerDeps, nil)
	}
	if publisher == nil {
		publisher = events.NoOp{}
	}
	if now == nil {
		now = time.Now
	}
	return &AccountSettler{repo: repo, tx: tx, ownership: ownership, unlinker: unlinker, publisher: publisher, now: now}, nil
}

// SettleAccount removes userID from every active Campaign it belongs to, before the account is deleted.
// An Owner hands ownership to the longest-standing GM; when no GM can take it, the Campaign enters the
// deleting state. Each Campaign is settled in its own transaction and every step re-reads the Campaign,
// so a retry after a crash finds only what is still left to do.
func (a *AccountSettler) SettleAccount(ctx context.Context, userID string) error {
	if err := requireUser(userID); err != nil {
		return err
	}
	found, err := a.repo.ListActiveByMember(ctx, userID)
	if err != nil {
		return internalError(MsgListFailed, err)
	}
	for i := range found {
		campaignID := found[i].ID
		if member, ok := found[i].MemberOf(userID); ok && member.Owner {
			if err := a.handOff(ctx, campaignID, userID); err != nil {
				return err
			}
		}
		if err := a.leave(ctx, campaignID, userID); err != nil {
			return err
		}
	}
	return nil
}

type handOffOutcome int

const (
	handOffSkipped handOffOutcome = iota + 1
	handOffTransferred
	handOffNoSuccessor
)

func (a *AccountSettler) handOff(ctx context.Context, campaignID, userID string) error {
	var outcome handOffOutcome
	var successor string
	out, applied, err := mutate(ctx, a.tx, a.repo, campaignID, a.now(), func(txCtx context.Context, c *Campaign) (bool, error) {
		outcome, successor = handOffSkipped, ""
		if ownerOf(c) != userID {
			return false, nil
		}
		candidate, err := a.pickSuccessor(txCtx, c, userID)
		if err != nil {
			return false, err
		}
		if candidate == "" {
			outcome = handOffNoSuccessor
			return false, nil
		}
		if err := a.ownership.ReleaseOwnership(txCtx, userID); err != nil {
			return false, internalError(MsgOwnershipWrite, err)
		}
		if err := transferOwnership(c, userID, candidate); err != nil {
			return false, err
		}
		outcome, successor = handOffTransferred, candidate
		return true, nil
	}, false)
	if err != nil {
		return err
	}
	switch {
	case applied && outcome == handOffTransferred:
		a.publishEvent(EventOwnershipTransferred, userID, out, map[string]any{
			dataKeyFromUserID: userID,
			dataKeyToUserID:   successor,
		})
		return nil
	case outcome == handOffNoSuccessor:
		return a.requestDeletion(ctx, campaignID)
	}
	return nil
}

func (a *AccountSettler) pickSuccessor(ctx context.Context, c *Campaign, userID string) (string, error) {
	for _, candidate := range successorCandidates(c, userID) {
		err := a.ownership.ReserveOwnership(ctx, candidate)
		if err == nil {
			return candidate, nil
		}
		if !hasCode(err, apperr.CodePlanLimitReached) {
			return "", err
		}
	}
	return "", nil
}

func (a *AccountSettler) requestDeletion(ctx context.Context, campaignID string) error {
	_, err := RequestDeletion(ctx, a.tx, a.repo, a.ownership, campaignID, a.now())
	if hasCode(err, apperr.CodeCampaignNotFound) {
		return nil
	}
	return err
}

func (a *AccountSettler) leave(ctx context.Context, campaignID, userID string) error {
	out, applied, err := mutate(ctx, a.tx, a.repo, campaignID, a.now(), func(txCtx context.Context, c *Campaign) (bool, error) {
		idx := c.indexOf(userID)
		if idx < 0 {
			return false, nil
		}
		member := c.Members[idx]
		if member.Owner {
			return false, internalError(MsgHandOffIncomplete, nil)
		}
		if err := a.unlinker.UnlinkCharacter(txCtx, c.ID, userID); err != nil {
			return false, internalError(MsgCampaignWrite, err)
		}
		c.Members = slices.Delete(c.Members, idx, idx+1)
		return true, nil
	}, false)
	if err != nil {
		return err
	}
	if applied {
		a.publishEvent(EventMemberLeft, userID, out, map[string]any{
			dataKeyUserID: userID,
			dataKeyReason: reasonLeft,
		})
	}
	return nil
}

func (a *AccountSettler) publishEvent(eventType, actorID string, c Campaign, data map[string]any) {
	actor := actorID
	a.publisher.Publish(events.Event{
		Type:             eventType,
		ActorID:          &actor,
		AggregateType:    aggregateType,
		AggregateID:      c.ID,
		AggregateVersion: c.Version,
		Data:             data,
	})
}

// successorCandidates lists the other GMs, longest-standing first. Ties on the join time go to the
// smaller user id so every retry picks the same successor.
func successorCandidates(c *Campaign, userID string) []string {
	var gms []Member
	for _, m := range c.Members {
		if m.Role == RoleGM && m.UserID != userID {
			gms = append(gms, m)
		}
	}
	slices.SortFunc(gms, func(a, b Member) int {
		if order := a.JoinedAt.Compare(b.JoinedAt); order != 0 {
			return order
		}
		return strings.Compare(a.UserID, b.UserID)
	})
	ids := make([]string, 0, len(gms))
	for _, m := range gms {
		ids = append(ids, m.UserID)
	}
	return ids
}

func transferOwnership(c *Campaign, from, to string) error {
	fromIdx, toIdx := -1, -1
	for i := range c.Members {
		switch c.Members[i].UserID {
		case from:
			fromIdx = i
		case to:
			toIdx = i
		}
	}
	if fromIdx < 0 || toIdx < 0 {
		return internalError(MsgHandOffIncomplete, nil)
	}
	c.Members[fromIdx].Owner = false
	c.Members[toIdx].Owner = true
	return nil
}

func hasCode(err error, code apperr.Code) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == code
}
