# Contract: The failure kinds and their copy

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirement IDs**: `WCS-11`, `WCS-12`,
`WCS-13` | **Owner**: `web/src/api/error_mapper.ts`, `web/src/api/errors.ts`,
`web/src/l10n/strings.ts`

## Purpose

Give the three new refusals a distinct interface-facing meaning and a distinct copy, so a wrong digit,
an exhausted send allowance and a duplicate address stop resolving to the same generic sentence — while
every existing classification and copy stays exactly as delivered.

## Shape

```ts
type FailureKind =
  | 'clientFault' | 'invalidEdit' | 'renderMismatch' | 'notFound' | 'serverError' | 'offline'
  | 'unauthorized' | 'forbidden' | 'rateLimited' | 'unknown'
  | 'invalidCode' | 'emailUnavailable' | 'conflict';   // NEW
```

| Code | Status | Kind | Copy key | Emitted by |
|---|---|---|---|---|
| `OTP_INVALID` | 401 | `invalidCode` | `api.error.invalidCode` | `POST /auth/challenge/verify` |
| `EMAIL_UNAVAILABLE` | 503 | `emailUnavailable` | `api.error.emailUnavailable` | `POST /auth/register` (gate on), `POST /auth/challenge/resend` |
| `CONFLICT` | 409 | `conflict` | `api.error.conflict` | `POST /auth/register` — duplicate address only |

## Rules

- The three codes join the existing `KIND_BY_CODE` table; no existing entry changes and no existing
  copy changes (`WCS-11`).
- The mapping stays total over the auth surface's codes: `INVALID_JSON`, `INVALID_REQUEST`,
  `UNKNOWN_FIELD`, `UNAUTHORIZED`, `FORBIDDEN`, `CONFLICT`, `RATE_LIMITED`, `EMAIL_UNAVAILABLE`,
  `OTP_INVALID`, `INTERNAL_ERROR`. A code outside that set keeps the delivered `unknown` fallback with
  its raw code and status preserved (`WCS-12`).
- Every copy resolves through `message(key)` in `src/l10n/strings.ts`; the API module keeps emitting
  codes and kinds and never authors text (`WCS-13`).
- `classifyDomainFailure` keeps rewriting the failure with the resolved kind while carrying the raw
  code, the status and the server message through unchanged.

## The copy

| Key | Copy | The next step it must name |
|---|---|---|
| `api.error.invalidCode` | That code is not valid or has expired. Check the newest email, or send a new code. | the newest email and the resend action — a stale message is the most common cause of a refused code (`WCS-11`) |
| `api.error.emailUnavailable` | We could not send the code right now. Try again in a moment. | wait, then resend; it must not disclose the account's state (`WCS-09`) |
| `api.error.conflict` | That email already has an account. Sign in instead. | switch to sign-in, through the inline action the refusal carries (`WCS-27`) |

## Invariants

1. **Distinct meaning.** No two of the three new kinds resolve to the same copy, and none of them
   resolves to the generic copy.
2. **No misclassification.** An unknown code never resolves to one of the classified kinds, and its raw
   code and status survive on the failure.
3. **No server text on screen.** The copy comes from the catalog, never from `error.message`, so a
   server sentence cannot reach a person.
4. **One table.** The classification lives only in `KIND_BY_CODE`; no screen and no operation parses a
   code.
5. **CONFLICT scope.** The `conflict` copy names the address because the server emits `CONFLICT` in
   exactly one place today; if another route starts emitting it, this copy must be revisited (`R6`).
6. **The offered recovery is offered.** The duplicate-address refusal carries an inline action that
   switches the credential form to sign-in, keeps the address and moves the focus to the password field,
   so the copy's promise can be taken (`WCS-27`). The action is part of the surface, not of the mapper:
   the mapper still emits a kind and a sentence only (`WCS-13`).

## Boundary cases

| Case | Result |
|---|---|
| `OTP_INVALID` | `invalidCode`, the invalid-code copy (`WCS-11`) |
| `EMAIL_UNAVAILABLE` | `emailUnavailable`, the unavailable copy (`WCS-11`) |
| `CONFLICT` | `conflict`, the duplicate-address copy (`WCS-11`) |
| `UNAUTHORIZED` | `unauthorized` — unchanged (`WCS-11`) |
| `RATE_LIMITED` | `rateLimited` — unchanged (`WCS-11`) |
| a code the table does not carry | `unknown`, raw code preserved (`WCS-12`) |
| a `DomainFailure` with no code | `unknown`, not coerced (`WCS-12`) |

## Test evidence

- `web/src/test/api_error_mapper.test.ts`: each new code's kind and resolved copy; the unknown fallback;
  the existing entries unchanged; the copy coming from the catalog rather than the server message.
- `web/src/test/l10n_strings.test.ts`: the new keys are declared and every scanned surface resolves
  through the catalog (`WCS-13`).
