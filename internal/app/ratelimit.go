package app

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"legnaris/internal/platform/ratelimit"
	"legnaris/shared/apperr"

	"github.com/labstack/echo/v5"
)

// MsgRateLimited is the user-facing message for a request refused by the rate
// limiter (AUTH-18).
const MsgRateLimited = "too many attempts; retry later"

// Observer receives the rate-limiter signal from the guard. It is an interface
// so the guard does not depend on the metrics implementation (AUTH-20).
type Observer interface {
	RateLimitBlocked()
}

// rateLimit refuses a request whose client address — and, when withAccount is
// set, whose account identity from the JSON body — exceeded its attempts for
// the window. A refusal answers 429 with a Retry-After header and never reaches
// the handler, so no credential is verified and no token is issued (AUTH-17,
// AUTH-18, AUTH-19).
func rateLimit(limiter *ratelimit.Limiter, observer Observer, withAccount bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			now := limiter.Now()

			if allowed, retryAfter := limiter.Allow(ipKey(c), now); !allowed {
				return rateLimited(c, observer, retryAfter)
			}

			if withAccount {
				if email := peekEmail(c.Request()); email != "" {
					if allowed, retryAfter := limiter.Allow(accountKey(email), now); !allowed {
						return rateLimited(c, observer, retryAfter)
					}
				}
			}

			return next(c)
		}
	}
}

// rateLimited sets Retry-After (whole seconds, rounded up), records the block
// when an observer is present, and returns the typed 429 (AUTH-18, AUTH-23). The
// observer is optional so a guard whose blocks must not be counted against the
// auth metric (the CSP report collector) can reuse the same refusal.
func rateLimited(c *echo.Context, observer Observer, retryAfter time.Duration) error {
	c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(retryAfterSeconds(retryAfter)))
	if observer != nil {
		observer.RateLimitBlocked()
	}
	return apperr.NewError(apperr.CodeRateLimited, MsgRateLimited, nil)
}

// retryAfterSeconds rounds the wait up to whole seconds with a floor of one, so
// a sub-second wait still advertises a Retry-After (AUTH-18).
func retryAfterSeconds(retryAfter time.Duration) int {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func ipKey(c *echo.Context) string {
	return "ip:" + c.RealIP()
}

func accountKey(email string) string {
	return "account:" + normalizeAccountKey(email)
}

// normalizeAccountKey mirrors the service's email normalization so both layers
// bucket the same account under the same key.
func normalizeAccountKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// peekEmail reads the email from the JSON body without consuming it: the bytes
// are put back so the handler still decodes the same request (AUTH-17). A body
// that is absent or not JSON simply yields no account key.
func peekEmail(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return normalizeAccountKey(payload.Email)
}
