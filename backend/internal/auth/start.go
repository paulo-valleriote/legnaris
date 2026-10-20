package auth

// StartOutcome is the discriminated result Register and Login hand back: the
// service decides whether a session was issued or a code step was started, and
// the controller only projects the decision (TFA-01, TFA-02, R1).
//
// Exactly one half is set on success — never both, never neither. The session
// half is filled when the rollout gate is disabled; the challenge half is
// filled when it is enabled, and it is filled only after the request's own
// credential proof succeeded (TFA-03).
type StartOutcome struct {
	// Session is the established session, when the gate is disabled.
	Session *Session
	// Challenge is the started code step, when the gate is enabled. It carries
	// the identity the client presents back, the claimed lifetime and the
	// resend window (OTP-10).
	Challenge *ChallengeIssued
}
