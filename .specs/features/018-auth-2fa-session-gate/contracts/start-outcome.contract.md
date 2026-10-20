# Contract: Start outcome and the single gate decision

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | Requirements: `TFA-01`, `TFA-02`, `TFA-03`, `TFA-05`, `TFA-07`, `TFA-11`, `TFA-12`, `TFA-13`

The seam between `internal/auth` and `internal/app`: the service decides whether a session or a code
step was started, and the controller only projects the decision (`R1`).

## Shape

```go
// internal/auth/start.go
type StartOutcome struct {
    Session   *Session         // set when the gate is disabled
    Challenge *ChallengeIssued // set when the gate is enabled
}
```

`Register(ctx, RegisterCommand) (*StartOutcome, error)` and `Login(ctx, LoginCommand) (*StartOutcome, error)`.
`Session` and `ChallengeIssued` keep their existing shapes (`AUTH-01`, `OTP-10`) — one new type is
added, nothing is redefined.

## The decision

```text
Register: validate email/username/password ─▶ hash ─▶ Create(account)
             └─ gate off ─▶ signIn ─▶ Session
             └─ gate on  ─▶ IssueChallenge{UserID, Email, PurposeVerifyEmail} ─▶ Challenge

Login:    canonicalize email ─▶ FetchByEmail
             ├─ not found ─▶ dummy password compare ─▶ uniform 401
             └─ found ─▶ Compare(password)
                    ├─ mismatch ─▶ uniform 401
                    └─ match ─▶ gate off ─▶ signIn ─▶ Session
                                 gate on  ─▶ IssueChallenge{UserID, Email, PurposeLoginOTP} ─▶ Challenge
```

The purpose is bound to the operation at the call site and never crosses the HTTP boundary
(`TFA-03`).

## Invariants

1. **Exactly one half** on success (`nil`, nil) is an error, not a state: `Session != nil XOR
   Challenge != nil`.
2. **Proof before issuance** (`TFA-03`): `IssueChallenge` is reached only after the request's own
   credential proof succeeded, so no unauthenticated request can cause a send.
3. **No new session path** (`TFA-11`): `signIn` is the only issuer; the gated branch never mints a
   session, and `VerifyChallenge` remains the only gated route to one.
4. **Uniform failure precedes the decision** (`TFA-05`): the dummy-digest comparison happens on the
   unknown-address path before the uniform 401 is produced, so both credential failures cost the same
   work. The dummy digest is a package-level bcrypt hash at the repository's cost and its result is
   discarded.
5. **Recoverability** (`TFA-13`): a failure from `IssueChallenge` (allowance spent → `EMAIL_UNAVAILABLE`
   503 with retry hint; store failure → `INTERNAL_ERROR` 500) propagates unchanged, with no session and
   no credential; the account keeps no verification timestamp and sign-in remains the recovery door
   (`R6`).
6. **Legacy parity** (`TFA-12`): with the gate disabled the returned `Session` and the errors are
   byte-identical to today's; the gate is the only input that changes the branch.

## Rejected shapes

- Two service methods per operation (`R1`): duplicates validation or moves policy into transport.
- `Session` returned with a nil-tolerance convention (a caller checking "challenge or session?" by a
  sentinel field): the explicit two-field result makes the invariant assertable.

## Tests

- Table over gate × {register, login} asserting which half is set.
- Gate on, unknown address vs wrong password: same typed code, same message, and the password
  comparison exercised on both (a `PasswordHasher` fake counts the calls).
- Gate on, `IssueChallenge` failing with each typed error: propagated, no session, account untouched.
- Gate off: `Session` equals the pre-existing expectations (the existing cases, run unchanged).
- Concurrency: two sign-ins for one account with the gate on produce two distinct challenges and only
  the later one verifies (`OTP-01`).
