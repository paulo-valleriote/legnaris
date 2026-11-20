package app

import (
	"net/http"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"

	"github.com/labstack/echo/v5"
)

const (
	funnelEventVisit      = "visit"
	funnelEventCTAClick   = "cta_click"
	funnelRateLimitKey    = "funnel:"
	MsgFunnelEventInvalid = "event must be visit or cta_click"
)

type FunnelObserver interface {
	HomeVisited()
	HomeCTAClicked()
}

type FunnelController struct {
	observer FunnelObserver
}

type funnelEventCommand struct {
	Event string `json:"event"`
}

func NewFunnelController(observer FunnelObserver) *FunnelController {
	return &FunnelController{observer: observer}
}

func (fc *FunnelController) Home(c *echo.Context) error {
	var cmd funnelEventCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	switch cmd.Event {
	case funnelEventVisit:
		fc.observer.HomeVisited()
	case funnelEventCTAClick:
		fc.observer.HomeCTAClicked()
	default:
		return apperr.NewError(apperr.CodeInvalidRequest, MsgFunnelEventInvalid, nil)
	}

	return c.NoContent(http.StatusNoContent)
}

func funnelRateLimit(limiter ratelimit.Allower) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if allowed, retryAfter := limiter.Allow(funnelRateLimitKey+ipKey(c), limiter.Now()); !allowed {
				return rateLimited(c, nil, retryAfter)
			}
			return next(c)
		}
	}
}
