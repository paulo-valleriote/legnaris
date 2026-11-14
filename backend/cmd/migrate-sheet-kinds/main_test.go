package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/sheet"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		migrate  func(context.Context) (sheet.MigrationReport, error)
		wantCode int
		wantOut  string
	}{
		{
			name: "Happy path: the report is printed and the run succeeds",
			migrate: func(context.Context) (sheet.MigrationReport, error) {
				return sheet.MigrationReport{BackfilledSheets: 3, Counters: 2}, nil
			},
			wantCode: 0,
			wantOut:  "sheets backfilled to kind character: 3\ncounters rebuilt: 2\n",
		},
		{
			name: "Edge: an already migrated database reports zeros and succeeds",
			migrate: func(context.Context) (sheet.MigrationReport, error) {
				return sheet.MigrationReport{}, nil
			},
			wantCode: 0,
			wantOut:  "sheets backfilled to kind character: 0\ncounters rebuilt: 0\n",
		},
		{
			name: "Error: a failed migration exits with 1 and says why",
			migrate: func(context.Context) (sheet.MigrationReport, error) {
				return sheet.MigrationReport{}, errors.New("replica set unreachable")
			},
			wantCode: 1,
			wantOut:  "migration failed: replica set unreachable\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			code := run(context.Background(), &out, tc.migrate)
			assert.Equal(t, tc.wantCode, code)
			assert.Equal(t, tc.wantOut, out.String())
		})
	}
}
