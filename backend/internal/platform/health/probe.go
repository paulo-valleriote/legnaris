package health

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Probe issues a GET to url and succeeds only on a 200 within timeout. It is
// the in-binary container healthcheck: the distroless runtime image ships no
// shell, curl or wget.
func Probe(ctx context.Context, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build probe request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe %s: unexpected status %d", url, resp.StatusCode)
	}
	return nil
}
