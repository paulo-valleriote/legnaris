package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/logger"

	"github.com/labstack/echo/v5"
)

// cspReportMaxBody caps a violation report body. A browser sends one small
// JSON document per violation, so 16 KB is generous; a body past the cap is
// dropped instead of buffered (contracts/csp-report.contract.md).
const cspReportMaxBody = 16 * 1024

// cspReportsMediaType is the `report-to` shape (an array); the legacy
// `csp-report` content type and a plain `application/json` body are tolerated
// and normalised through the same reader (A2).
const cspReportsMediaType = "application/reports+json"

// cspReportLogMessage is the one message the controller emits per collected
// violation, so a log query can isolate reports (A5).
const cspReportLogMessage = "csp violation report"

// cspUnknownLabel replaces an empty label value so the metric never carries a
// blank series an operator cannot read (A6).
const cspUnknownLabel = "unknown"

// CSPReportObserver receives the CSP violation signal. It is an interface so
// the controller does not depend on the metrics implementation, mirroring the
// rate-limiter Observer (AUTH-20).
type CSPReportObserver interface {
	CSPReportReported(effectiveDirective, disposition string)
}

// CSPReportController collects CSP violation reports: it normalises the two
// wire formats, sanitises the URLs, logs the violation, counts it and answers
// 204 unconditionally — a report is fire-and-forget and must never surface an
// error to the browser (POL-04, POL-05, csp-report.contract.md).
type CSPReportController struct {
	log      logger.StructuredLogger
	observer CSPReportObserver
	now      func() time.Time
}

// NewCSPReportController builds the controller over its logger and metric
// observer.
func NewCSPReportController(log logger.StructuredLogger, observer CSPReportObserver) *CSPReportController {
	return &CSPReportController{
		log:      log,
		observer: observer,
		now:      time.Now,
	}
}

// Report reads a capped body, normalises every violation it carries, and
// answers 204. A body that cannot be read or decoded is logged at error level
// and still answers 204: the browser must not retry a report, and an internal
// failure is never a 500 (A3, A8).
func (cc *CSPReportController) Report(c *echo.Context) error {
	ctx := c.Request().Context()

	body := http.MaxBytesReader(c.Response(), c.Request().Body, cspReportMaxBody)
	defer func() { _ = body.Close() }()

	payload, err := io.ReadAll(body)
	if err != nil {
		cc.log.Error(ctx, "csp report body rejected",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return c.NoContent(http.StatusNoContent)
	}

	mediaType, _, _ := mime.ParseMediaType(c.Request().Header.Get(echo.HeaderContentType))
	reports, err := decodeCSPReports(mediaType, payload, cc.now())
	if err != nil {
		cc.log.Error(ctx, "csp report decode failed",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return c.NoContent(http.StatusNoContent)
	}

	for _, report := range reports {
		cc.collect(ctx, report)
	}

	return c.NoContent(http.StatusNoContent)
}

// collect logs one normalised violation and counts it. The URLs reach the log
// already stripped of query and fragment, so a token an application put in a
// URL never lands in the log (A4, A5).
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

// cspReport is the single shape both wire formats collapse into.
type cspReport struct {
	EffectiveDirective string
	BlockedURI         string
	DocumentURI        string
	Disposition        string
	Timestamp          string
}

// cspReportEnvelope is the legacy `application/csp-report` body.
type cspReportEnvelope struct {
	Report cspReportBody `json:"csp-report"`
}

// cspReportBody is the violation object inside the legacy envelope.
type cspReportBody struct {
	DocumentURI        string `json:"document-uri"`
	BlockedURI         string `json:"blocked-uri"`
	EffectiveDirective string `json:"effective-directive"`
	ViolatedDirective  string `json:"violated-directive"`
	Disposition        string `json:"disposition"`
}

// reportToItem is one element of the `application/reports+json` array.
type reportToItem struct {
	Type string       `json:"type"`
	Body reportToBody `json:"body"`
}

// reportToBody is the violation object of the `report-to` shape.
type reportToBody struct {
	DocumentURL        string `json:"documentURL"`
	BlockedURL         string `json:"blockedURL"`
	EffectiveDirective string `json:"effectiveDirective"`
	Disposition        string `json:"disposition"`
}

// cspViolationType is the `report-to` item type; an item of any other type is
// not a CSP violation and is skipped.
const cspViolationType = "csp-violation"

// decodeCSPReports normalises either wire format to the single cspReport shape.
// `application/reports+json` (and any array body) is read as a list; the legacy
// envelope and a plain `application/json` object are read as one report. A body
// that decodes to nothing meaningful yields no reports rather than an error
// (A2).
func decodeCSPReports(mediaType string, body []byte, at time.Time) ([]cspReport, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if mediaType == cspReportsMediaType || trimmed[0] == '[' {
		var items []reportToItem
		if err := json.Unmarshal(trimmed, &items); err == nil {
			return reportsFromReportTo(items, at), nil
		} else if trimmed[0] == '[' {
			return nil, err
		}
	}

	var envelope cspReportEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Report.empty() {
		return []cspReport{envelope.Report.normalized(at)}, nil
	}

	var single reportToItem
	if err := json.Unmarshal(trimmed, &single); err == nil && !single.Body.empty() {
		return []cspReport{single.Body.normalized(at)}, nil
	}

	return nil, nil
}

// reportsFromReportTo normalises the items of a `report-to` array, dropping the
// ones that are not CSP violations and the ones that carry no violation data.
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

// empty reports whether the legacy body carries no violation field at all, which
// is what separates an empty envelope from a real report.
func (b cspReportBody) empty() bool {
	return b.DocumentURI == "" && b.BlockedURI == "" &&
		b.EffectiveDirective == "" && b.ViolatedDirective == "" && b.Disposition == ""
}

// normalized projects the legacy body onto the single report shape. The older
// `violated-directive` stands in when the modern field is absent, and both URLs
// are sanitised (A4).
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

// empty reports whether the `report-to` body carries no violation field.
func (b reportToBody) empty() bool {
	return b.DocumentURL == "" && b.BlockedURL == "" &&
		b.EffectiveDirective == "" && b.Disposition == ""
}

// normalized projects the `report-to` body onto the single report shape.
func (b reportToBody) normalized(at time.Time) cspReport {
	return cspReport{
		EffectiveDirective: b.EffectiveDirective,
		BlockedURI:         sanitizeURI(b.BlockedURL),
		DocumentURI:        sanitizeURI(b.DocumentURL),
		Disposition:        b.Disposition,
		Timestamp:          at.UTC().Format(time.RFC3339),
	}
}

// sanitizeURI drops the query and the fragment from a URL so a token or an
// identifier carried in a URL never reaches the log (A4). A value the parser
// cannot read is truncated at the first `?` or `#` instead of logged raw.
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

// labelValue replaces an empty value with a stable label so the metric series
// stays readable (A6).
func labelValue(value string) string {
	if value == "" {
		return cspUnknownLabel
	}
	return value
}

// cspRateLimit refuses a report past its address cap, answering 429 with
// Retry-After. Reports sit under their own key namespace (A7): a page emitting
// a burst of violations must never consume the auth budget, and an auth burst
// must never silence reporting.
func cspRateLimit(limiter *ratelimit.Limiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if allowed, retryAfter := limiter.Allow(cspIPKey(c), limiter.Now()); !allowed {
				// Reports are not auth traffic, so the block is deliberately not
				// counted against the auth rate-limit metric (AUTH-20).
				return rateLimited(c, nil, retryAfter)
			}
			return next(c)
		}
	}
}

// cspIPKey namespaces the report counter under its own prefix so the key space
// never collides with the auth guard's (A7).
func cspIPKey(c *echo.Context) string {
	return "csp:" + ipKey(c)
}
