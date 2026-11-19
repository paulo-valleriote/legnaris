package app

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	sharedjson "legnaris/backend/shared/json"
)

const MsgRateLimited = "too many attempts; retry later"

type Observer interface {
	RateLimitBlocked()
}

func rateLimit(ipLimiter, accountLimiter ratelimit.Allower, observer Observer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if allowed, retryAfter := ipLimiter.Allow(ipKey(c), ipLimiter.Now()); !allowed {
				return rateLimited(c, observer, retryAfter)
			}

			if accountLimiter != nil {
				if email := peekEmail(c.Request()); email != "" {
					if allowed, retryAfter := accountLimiter.Allow(accountKey(email), accountLimiter.Now()); !allowed {
						return rateLimited(c, observer, retryAfter)
					}
				}
			}

			return next(c)
		}
	}
}

func registerRateLimit(ipLimiter, emailLimiter ratelimit.Allower, observer Observer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if allowed, retryAfter := ipLimiter.Allow(ipKey(c), ipLimiter.Now()); !allowed {
				return rateLimited(c, observer, retryAfter)
			}

			if email := peekEmail(c.Request()); email != "" {
				if allowed, retryAfter := emailLimiter.Allow(accountKey(email), emailLimiter.Now()); !allowed {
					return rateLimited(c, observer, retryAfter)
				}
			}

			return next(c)
		}
	}
}

func rateLimited(c *echo.Context, observer Observer, retryAfter time.Duration) error {
	c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(retryAfterSeconds(retryAfter)))
	if observer != nil {
		observer.RateLimitBlocked()
	}
	return apperr.NewError(apperr.CodeRateLimited, MsgRateLimited, nil)
}

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
	return "account:" + canonicalAccountEmail(email)
}

func canonicalAccountEmail(email string) string {
	canonical, err := user.CanonicalEmail(email)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(email))
	}
	return canonical
}

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
	if err := sharedjson.DecodeBytes(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Email)
}

func userRateLimit(limiter ratelimit.Allower) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, ok := auth.UserID(c)
			if !ok {
				return next(c)
			}
			if allowed, retryAfter := limiter.Allow("user:"+userID, limiter.Now()); !allowed {
				return rateLimited(c, nil, retryAfter)
			}
			return next(c)
		}
	}
}
