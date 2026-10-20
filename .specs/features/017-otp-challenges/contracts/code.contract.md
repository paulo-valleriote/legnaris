# Contract: Code generation and digest

**Feature**: `.specs/features/017-otp-challenges/` | **Requirement IDs**: `OTP-02`, `OTP-03`,
`OTP-18`, `OTP-21` | **Owner**: `internal/auth` (`code.go`)

## Purpose

One place that produces the six-digit code and one place that turns it into the value the challenge
stores, so generation cannot drift from comparison and the identity binding that stops cross-account
and cross-purpose reuse is structural.

## Interface

```go
// minCodeSecretLength is the shortest accepted digest secret.
const minCodeSecretLength = 32

// codeLength is the fixed number of decimal digits.
const codeLength = 6

// CodeDigest keys the code digest. Constructed once at boot from the required
// EMAIL_CODE_SECRET; an absent, blank or short secret is a typed error (OTP-18).
type CodeDigest struct { /* unexported key */ }

func NewCodeDigest(secret string) (*CodeDigest, error)      // ErrCodeSecretWeak
func (d *CodeDigest) Digest(userID string, purpose Purpose, code string) string
func (d *CodeDigest) Equal(a, b string) bool

// CodeGenerator produces one code (OTP-02).
type CodeGenerator interface{ Generate() (string, error) }

func NewCodeGenerator() CodeGenerator                       // crypto/rand-backed
```

## Rules

- `Generate` returns exactly six characters `'0'`–`'9'`, uniform over the million values including
  leading zeros; it never returns fewer digits, never a non-digit, and the value is only used as a
  string (`OTP-02`).
- The source is `crypto/rand`; `math/rand` is never used (the plan's §3.4 invariant).
- `Digest` returns lowercase hex of
  `HMAC-SHA256(secret, "legnaris-otp-v1" ‖ 0x00 ‖ userID ‖ 0x00 ‖ purpose ‖ 0x00 ‖ code)`. The
  version tag lets a future algorithm change be distinguishable; the NUL separators make the binding
  unambiguous even if a field ever contains a separator character (`OTP-03`).
- `Equal` compares two digests with `crypto/hmac.Equal`, so a wrong code costs the same time as a
  right one regardless of the shared prefix (`OTP-03`).
- `NewCodeDigest` rejects an empty, blank (whitespace-only), or shorter-than-32-byte secret with
  `ErrCodeSecretWeak`, so the process that digests never starts without a real key (`OTP-18`).
- The secret is distinct from `JWT_SECRET` and is never reused for any other purpose (F-09,
  `OTP-21`); nothing in this package logs the secret or the code.

## Invariants

- Two challenges with the same code but different `user_id` or `purpose` have different digests.
- Two challenges with the same `user_id`, `purpose` and code have the same digest (deterministic).
- `Equal` is a constant-time comparison over equal-length hex strings, and it is never called with the
  raw code.
- The code never leaves this package except into the send-intention payload the caller supplies.

## Boundary cases

| Case | Result |
|---|---|
| code `000000` | generated uniformly, preserved with its leading zeros, digest computed on the six characters (`OTP-02`) |
| digest of the same code for `verify_email` vs `login_otp` | different values (`OTP-03`) |
| digest of the same code for two different accounts | different values (`OTP-03`) |
| secret of 31 bytes | `ErrCodeSecretWeak` at construction (`OTP-18`) |
| secret of 32 bytes | accepted |
| secret `""` or `"   "` | `ErrCodeSecretWeak` at construction (`OTP-18`) |
| comparison with a digest of different length | `false`, and no panic |
| generator failure (entropy source error) | returned to the caller, which answers `INTERNAL_ERROR`; no challenge is inserted (`OTP-17`) |

## Test evidence

- `internal/auth/code_test.go`: 10,000 generated codes match `^\d{6}$` and exercise a leading zero;
  the digest is deterministic; the identity and purpose binding changes the digest; `Equal` matches
  only the equal digest; `NewCodeDigest` rejects empty/blank/short and accepts exactly 32 bytes.
