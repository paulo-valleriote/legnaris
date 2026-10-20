# Contract: Templates

**Feature**: `.specs/features/016-email-infrastructure/` | **Requirement IDs**: `EML-11`, `EML-13`,
`EML-22` | **Owner**: `internal/email`

## Purpose

Turn a purpose and its data into a subject, a plain-text body and an HTML body that carry the code
and its lifetime — and never leak the code into the subject.

## Interface

```go
type Purpose string

const (
    PurposeVerifyEmail Purpose = "verify_email"
    PurposeLoginOTP    Purpose = "login_otp"
)

type Data struct {
    Code             string
    ExpiresInMinutes int
}

type Rendered struct {
    Subject string
    Text    string
    HTML    string
}

type Renderer interface {
    Render(purpose Purpose, data Data) (Rendered, error)
}

var ErrUnknownTemplate = errors.New("unknown email template")
```

## Files (embedded with `//go:embed templates/*`)

| Purpose | Subject | Text | HTML |
|---|---|---|---|
| `verify_email` | `templates/verify_email.subject.txt` | `templates/verify_email.body.txt` | `templates/verify_email.body.html` |
| `login_otp` | `templates/login_otp.subject.txt` | `templates/login_otp.body.txt` | `templates/login_otp.body.html` |

The subject and text are parsed with `text/template`; the HTML body with `html/template` (so the
code is escaped there). The copy is English.

## Rules

- `Render` selects the pair of files by `purpose`; an unknown purpose returns `ErrUnknownTemplate`
  and no send is attempted (permanent) (EML-11).
- The subject names the product and the action and never contains the code (EML-11).
- Both bodies contain the code (verbatim in text, escaped in HTML) and the lifetime
  (`expires_in_minutes`) (EML-11).
- Rendering is pure: the same `(purpose, data)` produces the same three strings.

## Invariants

- For every known purpose, `strings.Contains(rendered.Subject, data.Code)` is false.
- For every known purpose, both `Text` and `HTML` contain `data.Code` and the lifetime.
- `ErrUnknownTemplate` is the only error `Render` returns.

## Boundary cases

| Case | Result |
|---|---|
| `verify_email` and `login_otp` | each renders its own subject and bodies |
| unknown purpose (`""`, `"reset_password"`) | `ErrUnknownTemplate`, empty `Rendered` |
| code with an HTML-significant character (`<`, `&`) | escaped in `HTML`, verbatim in `Text` (the real generator emits digits only; the test proves the escaping) |
| leading-zero code | rendered verbatim (`"042917"`) |
| zero/negative `ExpiresInMinutes` | rendered as given (the caller guarantees positivity; no template-side validation) |

## Test evidence

- `internal/email/template_test.go`: per-purpose subject and both bodies; the code absent from the
  subject; the code and lifetime present in both bodies; unknown purpose; HTML escaping;
  determinism.
- `internal/email/message_test.go`: the rendered message carries the code only inside `data`.