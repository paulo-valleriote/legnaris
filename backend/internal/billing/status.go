package billing

import "legnaris/backend/internal/plan"

const StatusPastDue Status = "past_due"

func Paid(status Status) bool {
	return status == StatusActive
}

func DerivePlanID(subs []Subscription) string {
	for _, sub := range subs {
		if Paid(sub.Status) {
			return plan.IDPaid
		}
	}
	return plan.IDFree
}
