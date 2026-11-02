package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWait = 5 * time.Second

type serveHandle struct {
	url    string
	cancel context.CancelFunc
	result chan error
}

func startServe(t *testing.T, handler http.Handler, timeout time.Duration) *serveHandle {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	h := &serveHandle{url: "http://" + ln.Addr().String(), cancel: cancel, result: make(chan error, 1)}
	go func() { h.result <- serve(ctx, handler, ln, timeout) }()
	return h
}

func (h *serveHandle) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-h.result:
		return err
	case <-time.After(testWait):
		t.Fatal("serve did not return in time")
		return nil
	}
}

func get(url string) (int, error) {
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: testWait}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func blockingHandler(started chan<- struct{}, release <-chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	})
}

func TestServe(t *testing.T) {
	t.Run("Happy path: an in-flight request completes after the context is cancelled", func(t *testing.T) {
		started, release := make(chan struct{}, 1), make(chan struct{})
		h := startServe(t, blockingHandler(started, release), testWait)

		status := make(chan int, 1)
		go func() {
			code, _ := get(h.url)
			status <- code
		}()
		<-started

		h.cancel()
		close(release)

		assert.Equal(t, http.StatusOK, <-status)
		assert.NoError(t, h.wait(t))
	})

	t.Run("Edge: a new connection is refused once shutdown has begun", func(t *testing.T) {
		started, release := make(chan struct{}, 1), make(chan struct{})
		h := startServe(t, blockingHandler(started, release), testWait)

		go func() { _, _ = get(h.url) }()
		<-started

		h.cancel()
		require.Eventually(t, func() bool {
			_, err := get(h.url)
			return err != nil
		}, testWait, 10*time.Millisecond)

		close(release)
		assert.NoError(t, h.wait(t))
	})

	t.Run("Edge: an idle server returns nil as soon as the context is cancelled", func(t *testing.T) {
		h := startServe(t, http.NotFoundHandler(), testWait)

		h.cancel()

		assert.NoError(t, h.wait(t))
	})

	t.Run("Edge: a context cancelled before serving starts still shuts down cleanly", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.NoError(t, serve(ctx, http.NotFoundHandler(), ln, testWait))
	})

	t.Run("Error: a request outliving the timeout is forced closed and serve fails", func(t *testing.T) {
		started, release := make(chan struct{}, 1), make(chan struct{})
		t.Cleanup(func() { close(release) })
		h := startServe(t, blockingHandler(started, release), 50*time.Millisecond)

		clientErr := make(chan error, 1)
		go func() {
			_, err := get(h.url)
			clientErr <- err
		}()
		<-started

		h.cancel()

		require.ErrorIs(t, h.wait(t), context.DeadlineExceeded)
		assert.Error(t, <-clientErr)
	})

	t.Run("Error: a closed listener makes serve fail without waiting for the context", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		require.NoError(t, ln.Close())

		assert.Error(t, serve(context.Background(), http.NotFoundHandler(), ln, testWait))
	})
}
