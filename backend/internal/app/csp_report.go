package app

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"

	"github.com/labstack/echo/v5"
)

const cspReportMaxBody = 16 * 1024

const cspReportsMediaType = "application/reports+json"

const cspReportLogMessage = "csp violation report"

const cspUnknownLabel = "unknown"

type CSPReportObserver interface {
	CSPReportReported(effectiveDirective, disposition string)
}

type CSPReportController struct {
	log      logger.StructuredLogger
	observer CSPReportObserver
	now      func() time.Time
}

func NewCSPReportController(log logger.StructuredLogger, observer CSPReportObserver) *CSPReportController {
	return &CSPReportController{
		log:      log,
		observer: observer,
		now:      time.Now,
	}
}

func (cc *CSPReportController) Report(c *echo.Context) error {
	ctx := c.Request().Context()

	body := http.MaxBytesReader(c.Response(), c.Request().Body, cspReportMaxBody)
	defer func() { _ = body.Close() }()

	payload, err := io.ReadAll(body)
	if err != nil {
		cc.log.Error(ctx, "csp report body rejected",
			logger.Attribute{Key: logErrorKey, Value: err.Error()},
		)
		return c.NoContent(http.StatusNoContent)
	}

	mediaType, _, _ := mime.ParseMediaType(c.Request().Header.Get(echo.HeaderContentType))
	reports, err := decodeCSPReports(mediaType, payload, cc.now())
	if err != nil {
		cc.log.Error(ctx, "csp report decode failed",
			logger.Attribute{Key: logErrorKey, Value: err.Error()},
		)
		return c.NoContent(http.StatusNoContent)
	}

	for _, report := range reports {
		cc.collect(ctx, report)
	}

	return c.NoContent(http.StatusNoContent)
}

func (cc *CSPReportController) collect(ctx context.Context, report cspReport) {
	directive := labelValue(report.EffectiveDirective)
	disposition := labelValue(report.Disposition)

	cc.observer.CSPReportReported(directive, disposition)

	cc.log.Info(ctx, cspReportLogMessage,
		logger.Attribute{Key: "effective_directive", Value: directive},
		logger.Attribute{Key: "blocked_uri", Value: report.BlockedURI},
		logger.Attribute{Key: "document_uri", Value: report.DocumentURI},
		logger.Attribute{Key: "disposition", Value: disposition},
		logger.Attribute{Key: "timestamp", Value: report.Timestamp},
	)
}

type cspReport struct {
	EffectiveDirective string
	BlockedURI         string
	DocumentURI        string
	Disposition        string
	Timestamp          string
}

type cspReportEnvelope struct {
	Report cspReportBody `json:"csp-report"`
}

type cspReportBody struct {
	DocumentURI        string `json:"document-uri"`
	BlockedURI         string `json:"blocked-uri"`
	EffectiveDirective string `json:"effective-directive"`
	ViolatedDirective  string `json:"violated-directive"`
	Disposition        string `json:"disposition"`
}

type reportToItem struct {
	Type string       `json:"type"`
	Body reportToBody `json:"body"`
}

type reportToBody struct {
	DocumentURL        string `json:"documentURL"`
	BlockedURL         string `json:"blockedURL"`
	EffectiveDirective string `json:"effectiveDirective"`
	Disposition        string `json:"disposition"`
}

const cspViolationType = "csp-violation"

func decodeCSPReports(mediaType string, body []byte, at time.Time) ([]cspReport, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if mediaType == cspReportsMediaType || trimmed[0] == '[' {
		var items []reportToItem
		if err := sharedjson.DecodeBytes(trimmed, &items); err == nil {
			return reportsFromReportTo(items, at), nil
		} else if trimmed[0] == '[' {
			return nil, err
		}
	}

	var envelope cspReportEnvelope
	if err := sharedjson.DecodeBytes(trimmed, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Report.empty() {
		return []cspReport{envelope.Report.normalized(at)}, nil
	}

	var single reportToItem
	if err := sharedjson.DecodeBytes(trimmed, &single); err == nil && !single.Body.empty() {
		return []cspReport{single.Body.normalized(at)}, nil
	}

	return nil, nil
}

func reportsFromReportTo(items []reportToItem, at time.Time) []cspReport {
	reports := make([]cspReport, 0, len(items))
	for _, item := range items {
		if item.Type != "" && item.Type != cspViolationType {
			continue
		}
		if item.Body.empty() {
			continue
		}
		reports = append(reports, item.Body.normalized(at))
	}
	return reports
}

func (b cspReportBody) empty() bool {
	return b.DocumentURI == "" && b.BlockedURI == "" &&
		b.EffectiveDirective == "" && b.ViolatedDirective == "" && b.Disposition == ""
}

func (b cspReportBody) normalized(at time.Time) cspReport {
	directive := b.EffectiveDirective
	if directive == "" {
		directive = b.ViolatedDirective
	}
	return cspReport{
		EffectiveDirective: directive,
		BlockedURI:         sanitizeURI(b.BlockedURI),
		DocumentURI:        sanitizeURI(b.DocumentURI),
		Disposition:        b.Disposition,
		Timestamp:          at.UTC().Format(time.RFC3339),
	}
}

func (b reportToBody) empty() bool {
	return b.DocumentURL == "" && b.BlockedURL == "" &&
		b.EffectiveDirective == "" && b.Disposition == ""
}

func (b reportToBody) normalized(at time.Time) cspReport {
	return cspReport{
		EffectiveDirective: b.EffectiveDirective,
		BlockedURI:         sanitizeURI(b.BlockedURL),
		DocumentURI:        sanitizeURI(b.DocumentURL),
		Disposition:        b.Disposition,
		Timestamp:          at.UTC().Format(time.RFC3339),
	}
}

func sanitizeURI(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if parsed, err := url.Parse(trimmed); err == nil {
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		parsed.RawFragment = ""
		return parsed.String()
	}

	if cut := strings.IndexAny(trimmed, "?#"); cut >= 0 {
		return trimmed[:cut]
	}
	return trimmed
}

func labelValue(value string) string {
	if value == "" {
		return cspUnknownLabel
	}
	return value
}

func cspRateLimit(limiter ratelimit.Allower) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if allowed, retryAfter := limiter.Allow(cspIPKey(c), limiter.Now()); !allowed {
				return rateLimited(c, nil, retryAfter)
			}
			return next(c)
		}
	}
}

func cspIPKey(c *echo.Context) string {
	return "csp:" + ipKey(c)
}
