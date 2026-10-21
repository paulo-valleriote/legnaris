# Contract: The auth operations and the sign-in outcome

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirement IDs**: `WCS-01`, `WCS-03`,
`WCS-07`, `WCS-15` | **Owner**: `web/src/api/auth_api.ts`

## Purpose

Fix how the client tells the two credential answers apart and what the two new operations send, so the
contract is read in one place, no screen parses a payload, and the credential posture stays a single
explicit decision per request.

## Surface

| Method & path | Body | Success | `credentials` | CSRF echo | Bearer |
|---|---|---|---|---|---|
| `POST /auth/register` | `{email, username, password}` | `201` → `SignInOutcome` | `include` | no | no |
| `POST /auth/login` | `{email, password}` | `200`/`202` → `SignInOutcome` | `include` | no | no |
| `POST /auth/challenge/verify` | `{challenge_id, code}` | `200` → `SessionBody` | `include` | no | no |
| `POST /auth/challenge/resend` | `{challenge_id}` | `202` → `ResendResult` | `include` | no | no |
| `POST /auth/refresh` | — | `200` → `SessionBody` | `include` | **yes** | no |
| `POST /auth/logout` | — | `204` → void | `include` | **yes** | no |

All six go through `createCaller` over the **raw** transport. The raw transport is the seam that
guarantees `/auth/*` never carries a Bearer header and never arms the refresh path (`WAS-16`).

## Shape

```ts
type SignInOutcome =
  | { readonly kind: 'session';   readonly session: SessionBody }
  | { readonly kind: 'challenge'; readonly challenge: ChallengeStep };

interface ChallengeStep {
  readonly challengeId: string;
  readonly expiresIn: number;
  readonly resendAfter: number;
}

function decodeSignInOutcome(data: unknown): SignInOutcome;
function decodeResend(data: unknown): ResendResult;
```

## Rules

- `decodeSignInOutcome` reads the payload's own keys: `access_token` present → a `'session'` outcome
  decoded by the existing `decodeSession`; otherwise `challenge_id` present → a `'challenge'` outcome
  decoded with all three of `challenge_id`, `expires_in`, `resend_after` required. Anything else is
  `drift(...)` → `DefensiveParseFailure`.
- The HTTP status is **never** consulted for the discrimination, because registration answers `201` in
  both gate states (`R1`).
- A code step is not a credential: decoding one writes nothing, holds nothing and returns no token.
- `verifyChallenge` sends the handle and the code verbatim — the code is not trimmed, padded,
  normalised or parsed as a number, so leading zeros survive (`WCS-03`).
- `resendChallenge` sends only the handle; it returns the advertised window.
- Every operation declares `credentials: 'include'` explicitly, so the verification answer's
  `Set-Cookie` is accepted on the cross-origin API (`R7`).
- Neither challenge operation sends `X-CSRF-Token`: the server applies no origin guard and reads no
  refresh cookie on those routes (`R7`).

## Invariants

1. **Exactly one member.** A successful credential answer yields `session != null XOR challenge != null`;
   a payload carrying both keys or neither throws a `DefensiveParseFailure` rather than picking one.
2. **No credential from a code step.** No code-step outcome carries, sets or exposes an access token,
   a refresh token or a CSRF token.
3. **Opaque handle.** The handle is carried as received and never parsed, validated or normalised by
   the client.
4. **Raw transport only.** No challenge operation is routed through the authenticated transport, so a
   Bearer header cannot be attached and a refresh cannot be triggered.
5. **CSRF scoping.** Exactly `refresh` and `logout` echo the CSRF token; the four others do not.

## Boundary cases

| Case | Result |
|---|---|
| `201` body with `access_token` | `'session'` outcome (`WCS-01`) |
| `201` body with `challenge_id` | `'challenge'` outcome (`WCS-01`) |
| body with both `access_token` and `challenge_id` | `DefensiveParseFailure` (`R1`) |
| body with neither | `DefensiveParseFailure` (`R1`) |
| code step missing `resend_after` | `DefensiveParseFailure` (`WCS-01`) |
| code `"042917"` | sent as `"042917"` (`WCS-03`) |
| `verifyChallenge` refused `401 OTP_INVALID` | `DomainFailure` classified `invalidCode` |
| `resendChallenge` refused `503 EMAIL_UNAVAILABLE` | `DomainFailure` classified `emailUnavailable` |
| register refused `409 CONFLICT` | `DomainFailure` classified `conflict` |
| any challenge call | no `Authorization`, no `X-CSRF-Token`, `credentials: include` (`WCS-15`) |

## Test evidence

- `web/src/test/auth_api.test.ts`: both outcomes decoded from the same operation; the ambiguous and the
  malformed payloads; the discrimination never reads the status; both new operations' method, path,
  body, credentials and absent headers; leading zeros preserved; the three refusals classified on the
  way out.
- `web/src/test/api_contract_captures.test.ts`: the seven code-step fixtures decode without a backend
  (`WCS-14`).
