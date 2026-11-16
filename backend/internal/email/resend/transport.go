package resend

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

const statusErrorBodyLimit = 512

type statusError struct {
	Status int
	Body   string
	Header http.Header
}

func (e *statusError) Error() string {
	return fmt.Sprintf("resend responded with status %d: %s", e.Status, e.Body)
}

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
	return nil, &statusError{
		Status: resp.StatusCode,
		Body:   strings.TrimSpace(string(body)),
		Header: resp.Header.Clone(),
	}
}
