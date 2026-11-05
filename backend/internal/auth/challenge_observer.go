package auth

// Closed result-label sets the ChallengeObserver records. Verification results
// include the internal classification of the uniform failure; resend results
// cover the three outcomes of a resend request (observability.contract.md).
const (
	ResultVerified  = "verified"
	ResultInvalid   = "invalid"
	ResultExpired   = "expired"
	ResultConsumed  = "consumed"
	ResultExhausted = "exhausted"
	ResultUnknown   = "unknown"

	ResendSent       = "sent"
	ResendSuppressed = "suppressed"
	ResendCapped     = "capped"
	// ResendReserved is a resend refused because the resend share of the day
	// is spent, while the total cap still has room for first sends (D5).
	ResendReserved = "reserved"
)

// ChallengeObserver receives the code-step signals. It is a separate seam from
// Observer so existing implementers and their generated mocks do not move for
// signals they do not care about (R10, OTP-16). No method carries a code, a
// challenge identity or an unmasked address (OTP-15).
type ChallengeObserver interface {
	ChallengeIssued(purpose Purpose)
	ChallengeVerified(result string)
	ResendRequested(result string)
	SendCapBlocked()
}

type nopChallengeObserver struct{}

func (nopChallengeObserver) ChallengeIssued(Purpose) {}

func (nopChallengeObserver) ChallengeVerified(string) {}

func (nopChallengeObserver) ResendRequested(string) {}

func (nopChallengeObserver) SendCapBlocked() {}

// NewNopChallengeObserver returns a ChallengeObserver that does nothing.
func NewNopChallengeObserver() ChallengeObserver {
	return nopChallengeObserver{}
}
