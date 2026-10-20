package resend

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// statusErrorBodyLimit caps how much of a failing provider response is retained
// for classification and diagnostics.
const statusErrorBodyLimit = 512

// statusError reports a non-2xx provider response the transport observed before
// the SDK's own error handling, preserving the status the SDK collapses into a
// plain message. It is what lets the adapter classify permanent vs retryable
// (EML-12).
type statusError struct {
	Status int
	Body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("resend responded with status %d: %s", e.Status, e.Body)
}

// statusTransport wraps the SDK's HTTP transport and turns any non-2xx response
// into a typed statusError. A 2xx response is returned untouched so the SDK can
// decode it; a failing body is drained, truncated and closed (EML-12).
type statusTransport struct {
	base http.RoundTripper
}

func (t statusTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return resp, nil
	}

	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, statusErrorBodyLimit))
	return nil, &statusError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
}
