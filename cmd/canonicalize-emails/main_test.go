package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"legnaris/internal/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cliIDOne = "01900000-0000-7000-8000-000000000201"
	cliIDTwo = "01900000-0000-7000-8000-000000000202"
)

// updateCall records one write the command asked the store to perform.
type updateCall struct {
	id        string
	email     string
	updatedAt time.Time
}

// fakeStore is the in-memory EmailMigrationStore the command body runs against.
type fakeStore struct {
	rows      []user.EmailIdentity
	listErr   error
	updateErr error
	updates   []updateCall
}

func (f *fakeStore) ListEmailIdentities(context.Context) ([]user.EmailIdentity, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rows, nil
}

func (f *fakeStore) UpdateEmail(_ context.Context, id string, email string, updatedAt time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updates = append(f.updates, updateCall{id: id, email: email, updatedAt: updatedAt})
	return nil
}

func runCLI(t *testing.T, args []string, store user.EmailMigrationStore) (int, string) {
	t.Helper()

	var stdout bytes.Buffer
	code := run(context.Background(), args, &stdout, store)
	return code, stdout.String()
}

// TestRun covers the documented report and exit codes of the operator command
// (PRQ-11, SC-008).
func TestRun(t *testing.T) {
	t.Run("Happy path: the default is a dry run that writes nothing", func(t *testing.T) {
		store := &fakeStore{rows: []user.EmailIdentity{
			{ID: cliIDOne, Email: "  Legacy Name <Legacy@Example.COM>  "},
		}}

		code, out := runCLI(t, nil, store)
		require.Equal(t, 0, code, out)

		assert.Contains(t, out, "scanned: 1\n")
		assert.Contains(t, out, "change: "+cliIDOne+"   Legacy Name <Legacy@Example.COM>   -> legacy@example.com\n")
		assert.Contains(t, out, "dry run: 1 changes not written\n")
		assert.Empty(t, store.updates, "a dry run must not write")
	})

	t.Run("Happy path: -apply writes one row and reports the count", func(t *testing.T) {
		store := &fakeStore{rows: []user.EmailIdentity{
			{ID: cliIDOne, Email: "Legacy@Example.COM"},
		}}

		code, out := runCLI(t, []string{"-apply"}, store)
		require.Equal(t, 0, code, out)

		assert.Contains(t, out, "scanned: 1\n")
		assert.Contains(t, out, "written: 1\n")
		assert.NotContains(t, out, "dry run:")

		require.Len(t, store.updates, 1)
		assert.Equal(t, cliIDOne, store.updates[0].id, "ids are never changed")
		assert.Equal(t, "legacy@example.com", store.updates[0].email)
		assert.False(t, store.updates[0].updatedAt.IsZero())
	})

	t.Run("Edge: already canonical data is a clean no-op", func(t *testing.T) {
		store := &fakeStore{rows: []user.EmailIdentity{
			{ID: cliIDOne, Email: "legacy@example.com"},
		}}

		code, out := runCLI(t, []string{"-apply"}, store)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "written: 0\n")
		assert.Empty(t, store.updates)
	})

	t.Run("Edge: an empty collection is a clean no-op", func(t *testing.T) {
		store := &fakeStore{}

		code, out := runCLI(t, nil, store)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "scanned: 0\n")
		assert.Contains(t, out, "dry run: 0 changes not written\n")
	})

	t.Run("Error: a collision blocks the run with exit 1 and no writes", func(t *testing.T) {
		store := &fakeStore{rows: []user.EmailIdentity{
			{ID: cliIDOne, Email: "Legacy@Example.COM"},
			{ID: cliIDTwo, Email: "legacy@example.com"},
		}}

		code, out := runCLI(t, []string{"-apply"}, store)
		assert.Equal(t, 1, code, out)
		assert.Contains(t, out, "collision: legacy@example.com "+cliIDOne+" "+cliIDTwo+"\n")
		assert.NotContains(t, out, "written:")
		assert.Empty(t, store.updates, "a blocked plan must write nothing")
	})

	t.Run("Error: an invalid row blocks the run with exit 1 and no writes", func(t *testing.T) {
		store := &fakeStore{rows: []user.EmailIdentity{
			{ID: cliIDOne, Email: "not-an-address"},
		}}

		code, out := runCLI(t, []string{"-apply"}, store)
		assert.Equal(t, 1, code, out)
		assert.Contains(t, out, "invalid: "+cliIDOne+" not-an-address (address cannot be parsed)\n")
		assert.Empty(t, store.updates)
	})

	t.Run("Error: a listing failure exits 1 and writes nothing", func(t *testing.T) {
		store := &fakeStore{listErr: errors.New("mongo down")}

		code, out := runCLI(t, []string{"-apply"}, store)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "error: list email identities: mongo down\n")
		assert.Empty(t, store.updates)
	})

	t.Run("Error: a mid-run write failure exits 1", func(t *testing.T) {
		store := &fakeStore{
			rows: []user.EmailIdentity{
				{ID: cliIDOne, Email: "Legacy@Example.COM"},
			},
			updateErr: errors.New("write failed"),
		}

		code, out := runCLI(t, []string{"-apply"}, store)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "error: update email for "+cliIDOne+": write failed\n")
		assert.NotContains(t, out, "written:")
	})

	t.Run("Error: an unknown flag is a usage error with exit 2", func(t *testing.T) {
		store := &fakeStore{}

		code, out := runCLI(t, []string{"-force"}, store)
		assert.Equal(t, 2, code)
		assert.Contains(t, out, "usage: canonicalize-emails [-apply]\n")
		assert.Empty(t, store.updates)
	})

	t.Run("Edge: -h prints the usage and exits 2", func(t *testing.T) {
		store := &fakeStore{}

		code, out := runCLI(t, []string{"-h"}, store)
		assert.Equal(t, 2, code)
		assert.Contains(t, out, "usage: canonicalize-emails [-apply]\n")
		assert.Empty(t, store.updates)
	})

	t.Run("Error: a positional argument is a usage error with exit 2", func(t *testing.T) {
		store := &fakeStore{}

		code, out := runCLI(t, []string{"-apply", "extra"}, store)
		assert.Equal(t, 2, code)
		assert.Contains(t, out, "usage: canonicalize-emails [-apply]\n")
		assert.Empty(t, store.updates)
	})

	t.Run("Edge: -apply=false keeps the dry-run behavior", func(t *testing.T) {
		store := &fakeStore{rows: []user.EmailIdentity{
			{ID: cliIDOne, Email: "Legacy@Example.COM"},
		}}

		code, out := runCLI(t, []string{"-apply=false"}, store)
		require.Equal(t, 0, code, out)
		assert.True(t, strings.Contains(out, "dry run: 1 changes not written\n"))
		assert.Empty(t, store.updates)
	})
}
