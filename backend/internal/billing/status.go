package billing

import (
	"time"

	"legnaris/backend/internal/plan"
)

const StatusPastDue Status = "past_due"

func Paid(status Status) bool {
	return status == StatusActive
}

func Entitled(sub Subscription, now time.Time) bool {
	if !Paid(sub.Status) {
		return false
	}
	return sub.Provider != ProviderComp || sub.CurrentPeriodEnd == nil || sub.CurrentPeriodEnd.After(now)
}

func IsExpiredComp(sub Subscription, now time.Time) bool {
	return sub.Provider == ProviderComp &&
		sub.Status == StatusActive &&
		sub.CurrentPeriodEnd != nil &&
		!sub.CurrentPeriodEnd.After(now)
}

func DerivePlanID(subs []Subscription, now time.Time) string {
	for _, sub := range subs {
		if Entitled(sub, now) {
			return plan.IDPaid
		}
	}
	return plan.IDFree
}
