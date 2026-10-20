package resend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOutbound() email.Outbound {
	return email.Outbound{
		IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		To:             "player@example.com",
		Subject:        "Your Legnaris login code",
		Text:           "code 042917",
		HTML:           "<p>code 042917</p>",
	}
}

// requestCapture records the request the adapter made, guarded so the server
// goroutine and the test goroutine never race.
type requestCapture struct {
	mu             sync.Mutex
	idempotencyKey string
	from           string
	to             []string
	subject        string
	text           string
	html           string
}

func (c *requestCapture) record(r *http.Request) {
	var body struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
		Html    string   `json:"html"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.idempotencyKey = r.Header.Get("Idempotency-Key")
	c.from = body.From
	c.to = body.To
	c.subject = body.Subject
	c.text = body.Text
	c.html = body.Html
}

func newTestSender(t *testing.T, handler http.HandlerFunc, timeout time.Duration) *Sender {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	sender, err := newWithBaseURL("test-key", "no-reply@example.com", server.URL, timeout)
	require.NoError(t, err)
	return sender
}

// TestSender_Success proves a 2xx provider response is a success and that the
// intention identity is sent as the idempotency key (EML-09).
func TestSender_Success(t *testing.T) {
	capture := &requestCapture{}
	sender := newTestSender(t, func(w http.ResponseWriter, r *http.Request) {
		capture.record(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"provider-id"}`))
	}, time.Second)

	outbound := testOutbound()
	require.NoError(t, sender.Send(context.Background(), outbound))

	t.Run("Happy path: the idempotency key is the intention identity", func(t *testing.T) {
		capture.mu.Lock()
		defer capture.mu.Unlock()
		assert.Equal(t, outbound.IdempotencyKey, capture.idempotencyKey)
	})

	t.Run("Happy path: the rendered message is what is sent", func(t *testing.T) {
		capture.mu.Lock()
		defer capture.mu.Unlock()
		assert.Equal(t, "no-reply@example.com", capture.from)
		assert.Equal(t, []string{outbound.To}, capture.to)
		assert.Equal(t, outbound.Subject, capture.subject)
		assert.Equal(t, outbound.Text, capture.text)
		assert.Equal(t, outbound.HTML, capture.html)
	})
}

// TestSender_Classification proves every provider outcome maps to the right
// sentinel (EML-12).
func TestSender_Classification(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantErr    error
		delay      time.Duration
		timeout    time.Duration
		closeFirst bool
	}{
		{name: "Happy path: 200 is a success", status: http.StatusOK},
		{name: "Error: 429 is transient", status: http.StatusTooManyRequests, wantErr: email.ErrTransient},
		{name: "Error: 500 is transient", status: http.StatusInternalServerError, wantErr: email.ErrTransient},
		{name: "Error: 503 is transient", status: http.StatusServiceUnavailable, wantErr: email.ErrTransient},
		{name: "Error: 400 is permanent", status: http.StatusBadRequest, wantErr: email.ErrPermanent},
		{name: "Error: 422 is permanent", status: http.StatusUnprocessableEntity, wantErr: email.ErrPermanent},
		{name: "Error: a send timeout is transient", status: http.StatusOK, wantErr: email.ErrTransient, delay: 200 * time.Millisecond, timeout: 20 * time.Millisecond},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = time.Second
			}
			sender := newTestSender(t, func(w http.ResponseWriter, _ *http.Request) {
				if tc.delay > 0 {
					time.Sleep(tc.delay)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"id":"provider-id"}`))
			}, timeout)

			err := sender.Send(context.Background(), testOutbound())

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// TestSender_NetworkError proves a transport failure is transient, so the
// pipeline retries rather than dropping a recoverable send (EML-12).
func TestSender_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	sender, err := newWithBaseURL("test-key", "no-reply@example.com", url, time.Second)
	require.NoError(t, err)

	err = sender.Send(context.Background(), testOutbound())
	require.Error(t, err)
	assert.ErrorIs(t, err, email.ErrTransient)
}

// TestNew_RejectsMissingConfiguration proves the adapter refuses to build
// without a key or a From address (EML-12).
func TestNew_RejectsMissingConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		from   string
	}{
		{name: "Error: empty API key", apiKey: "", from: "no-reply@example.com"},
		{name: "Error: blank API key", apiKey: "   ", from: "no-reply@example.com"},
		{name: "Error: empty From", apiKey: "key", from: ""},
		{name: "Error: blank From", apiKey: "key", from: "  "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender, err := New(tc.apiKey, tc.from, time.Second)
			require.Error(t, err)
			assert.Nil(t, sender)
			assert.ErrorIs(t, err, ErrMissingConfiguration)
		})
	}
}
