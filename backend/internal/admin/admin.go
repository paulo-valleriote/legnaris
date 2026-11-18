package admin

import (
	"context"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/db"
)

const MaxReasonLength = 500

const (
	MsgAdminUnavailable      = "admin operation is unavailable"
	MsgEmailRequired         = "email is required"
	MsgInvalidEmail          = "email is not valid"
	MsgReasonRequired        = "reason is required"
	MsgReasonTooLong         = "reason is too long"
	MsgUserNotFound          = "user not found"
	MsgInvalidUser           = "invalid user"
	MsgInvalidPlan           = "plan is not available for a comp"
	MsgExpiryNotFuture       = "expiresAt must be in the future"
	MsgLiveSubscriptionExist = "the user already has a live subscription"
	MsgCompNotFound          = "the user has no live comp subscription"
	MsgNotComp               = "the live subscription is not a comp"
	MsgCompStateChanged      = "the comp subscription state changed"
)

const (
	ActionCompGrant     = "comp.grant"
	ActionCompRevoke    = "comp.revoke"
	ActionQuotaReset    = "quota.reset"
	ActionRefundRequest = "refund.request"
)

type Entry struct {
	ID           string         `bson:"_id"`
	ActorUserID  string         `bson:"actor_user_id"`
	Action       string         `bson:"action"`
	TargetUserID string         `bson:"target_user_id"`
	Payload      map[string]any `bson:"payload,omitempty"`
	Reason       string         `bson:"reason"`
	CreatedAt    time.Time      `bson:"created_at"`
}

type UserView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type SubscriptionView struct {
	ID                string     `json:"id"`
	Provider          string     `json:"provider"`
	PlanID            string     `json:"plan_id"`
	Status            string     `json:"status"`
	CurrentPeriodEnd  *time.Time `json:"current_period_end"`
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
	RefundState       string     `json:"refund_state,omitempty"`
	RefundRequestedAt *time.Time `json:"refund_requested_at"`
	RefundUsedAt      *time.Time `json:"refund_used_at"`
}

type DailyQuotaView struct {
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetsAt  time.Time `json:"resets_at"`
}

type TrialView struct {
	Allowance int `json:"allowance"`
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

type View struct {
	User         UserView          `json:"user"`
	Plan         string            `json:"plan"`
	Subscription *SubscriptionView `json:"subscription"`
	DailyQuota   DailyQuotaView    `json:"daily_quota"`
	Trial        TrialView         `json:"trial"`
}

type CompGrant struct {
	PlanID    string
	ExpiresAt *time.Time
	Reason    string
}

type Users interface {
	FetchByID(ctx context.Context, id string) (*user.User, error)
	FetchByEmail(ctx context.Context, email string) (*user.User, error)
}

type Subscriptions interface {
	FindLive(ctx context.Context, userID string) (billing.Subscription, bool, error)
	InsertSubscription(ctx context.Context, sub billing.Subscription) error
	CancelComp(ctx context.Context, id string, now time.Time) (bool, error)
	RequestRefund(ctx context.Context, id string, now time.Time) (bool, error)
}

type Quotas interface {
	DailyStatus(ctx context.Context, userID string) (quota.DailyStatus, error)
	TrialStatus(ctx context.Context, userID string) (quota.TrialStatus, error)
	ResetDaily(ctx context.Context, userID string) error
}

type Plans interface {
	PlanOf(ctx context.Context, userID string) (plan.Plan, error)
}

type Audits interface {
	Insert(ctx context.Context, entry Entry) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Step(ctx context.Context, step db.Step) error
}

type Deps struct {
	Users         Users
	Subscriptions Subscriptions
	Quotas        Quotas
	Plans         Plans
	Audits        Audits
	Tx            Transactor
	Now           func() time.Time
}

func SubscriptionViewOf(sub billing.Subscription) SubscriptionView {
	return SubscriptionView{
		ID:                sub.ID,
		Provider:          sub.Provider,
		PlanID:            sub.PlanID,
		Status:            string(sub.Status),
		CurrentPeriodEnd:  sub.CurrentPeriodEnd,
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
		RefundState:       string(sub.RefundState),
		RefundRequestedAt: sub.RefundRequestedAt,
		RefundUsedAt:      sub.RefundUsedAt,
	}
}
