package resend

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestStatusTransport(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "https://api.example/emails", nil)
	require.NoError(t, err)

	t.Run("Happy path: a 2xx response passes through untouched", func(t *testing.T) {
		body := &trackingBody{Reader: strings.NewReader("ok")}
		base := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: body, Header: http.Header{}}, nil
		})

		resp, err := statusTransport{base: base}.RoundTrip(request)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.False(t, body.closed, "a success body must stay open for the SDK to decode")
	})

	t.Run("Happy path: a created response passes through", func(t *testing.T) {
		base := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusCreated, Body: &trackingBody{Reader: strings.NewReader("{}")}, Header: http.Header{}}, nil
		})

		resp, err := statusTransport{base: base}.RoundTrip(request)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("Error: a non-2xx response becomes a typed status error and closes the body", func(t *testing.T) {
		body := &trackingBody{Reader: strings.NewReader("boom")}
		base := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadGateway, Body: body, Header: http.Header{}}, nil
		})

		resp, err := statusTransport{base: base}.RoundTrip(request)

		require.Error(t, err)
		assert.Nil(t, resp)

		var statusErr *statusError
		require.True(t, errors.As(err, &statusErr), "the error must carry the provider status")
		assert.Equal(t, http.StatusBadGateway, statusErr.Status)
		assert.Equal(t, "boom", statusErr.Body)
		assert.True(t, body.closed, "a failing body must be closed")
	})

	t.Run("Edge: a large failing body is truncated to the limit", func(t *testing.T) {
		body := &trackingBody{Reader: strings.NewReader(strings.Repeat("x", 5000))}
		base := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: body, Header: http.Header{}}, nil
		})

		_, err := statusTransport{base: base}.RoundTrip(request)
		require.Error(t, err)

		var statusErr *statusError
		require.True(t, errors.As(err, &statusErr))
		assert.Len(t, statusErr.Body, statusErrorBodyLimit)
	})

	t.Run("Error: a transport-level error passes through unchanged", func(t *testing.T) {
		sentinel := errors.New("connection reset")
		base := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, sentinel })

		_, err := statusTransport{base: base}.RoundTrip(request)
		assert.ErrorIs(t, err, sentinel)
	})
}
