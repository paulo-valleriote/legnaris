# Contract: Canonical email

**Feature**: `.specs/features/015-auth-prerequisites/` | **Requirement IDs**: `PRQ-08`…`PRQ-11` |
**Owner**: `internal/user` (rule), `internal/auth` (usage), `internal/app` (account key)

## Purpose

One mailbox is one account however it is spelled, and the persisted value is deliverable.

## Rule

```go
const MaxEmailLength = 254

var ErrInvalidEmail = errors.New("email is invalid")

// CanonicalEmail returns the bare, lowercase, trimmed address.
func CanonicalEmail(raw string) (string, error)
```

Normative steps:

1. `raw = strings.TrimSpace(raw)`.
2. `parsed, err := mail.ParseAddress(raw)`; `err != nil` → `ErrInvalidEmail`.
3. `canonical := strings.ToLower(strings.TrimSpace(parsed.Address))`.
4. `len(canonical) > MaxEmailLength` → `ErrInvalidEmail`.
5. Return `canonical`.

Dots, `+tags`, underscores and every other local-part character are preserved. Display names,
surrounding whitespace and uppercase are removed. The length cap applies to the **canonical**
address.

## Consumers and outcomes

| Consumer | Input | Outcome |
|---|---|---|
| `auth.Register` | supplied address | Persist `canonical`; `ErrInvalidEmail` → 400 `INVALID_REQUEST` + `MsgInvalidEmail` (`PRQ-08`) |
| `auth.Login` | supplied address | `FetchByEmail(canonical)`; any canonicalization failure → the uniform 401 `INVALID_CREDENTIALS` (`PRQ-09`) |
| Duplicate detection | canonical value + `users_email_unique` | Second spelling is 409 `CONFLICT` + `MsgEmailAlreadyExists`, no second row (`PRQ-10`) |
| Rate-limit account key | JSON body `email` | `account:<canonical>`; on failure fall back to lower+trim (`AUTH-17`, `PRQ-09`) |
| Migration | stored `email` | Same function, so repair and creation cannot diverge (`PRQ-11`) |

## Invariants

- The stored value is exactly `CanonicalEmail(input)` for every row created after this feature.
- Equivalent spellings of one mailbox resolve to one account.
- An invalid or over-long address never reaches the store.
- The rule is pure, dependency-free and identical for create, sign-in, the key and the migration.

## Boundary cases

| Input | Result |
|---|---|
| `"  Player@Example.COM  "` | `player@example.com` |
| `"Name <Player@Example.com>"` | `player@example.com` |
| `player+news@example.com` | unchanged (tag preserved) |
| `p.l.a.y.e.r@example.com` | unchanged (dots preserved) |
| 254-byte canonical address | accepted |
| 255-byte canonical address | `ErrInvalidEmail` |
| `""`, `"   "`, `"player"`, `"player@"`, `"@example.com"`, two addresses | `ErrInvalidEmail` |

## Test evidence

- `internal/user/email_test.go`: the full table above, boundaries, quoted local part, and the
  no-transformation cases.
- `internal/auth/service_test.go`: stored value, invalid input, uniform sign-in failure.
- `internal/app/ratelimit_test.go`: equivalent spellings share one account key.
- `internal/app/integration_test.go` (Mongo, self-skip): two spellings → 409; sign-in with a
  display-name form reaches the same account.
