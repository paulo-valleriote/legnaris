package campaign_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"legnaris/backend/internal/campaign"
)

func TestSheetETag(t *testing.T) {
	tests := []struct {
		name          string
		sheetVersion  int64
		layoutVersion int64
		role          campaign.Role
		want          string
	}{
		{name: "Edge: zero versions for a pre-versioning sheet", sheetVersion: 0, layoutVersion: 0, role: campaign.RoleGM, want: `"0.0.gm"`},
		{name: "Edge: layout bump changes the tag without a sheet bump", sheetVersion: 5, layoutVersion: 1, role: campaign.RoleGM, want: `"5.1.gm"`},
		{name: "Happy path: sheet and layout versions with GM role", sheetVersion: 7, layoutVersion: 3, role: campaign.RoleGM, want: `"7.3.gm"`},
		{name: "Happy path: same versions for a player differ by role", sheetVersion: 7, layoutVersion: 3, role: campaign.RolePlayer, want: `"7.3.player"`},
		{name: "Edge: large counters", sheetVersion: 1 << 40, layoutVersion: 1 << 33, role: campaign.RoleGM, want: `"1099511627776.8589934592.gm"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, campaign.SheetETag(tc.sheetVersion, tc.layoutVersion, tc.role))
		})
	}
}

func TestSheetETag_ChangesWhenAnyInputChanges(t *testing.T) {
	base := campaign.SheetETag(4, 2, campaign.RoleGM)
	tests := []struct {
		name string
		etag string
	}{
		{name: "Edge: sheet version bump", etag: campaign.SheetETag(5, 2, campaign.RoleGM)},
		{name: "Edge: layout version bump", etag: campaign.SheetETag(4, 3, campaign.RoleGM)},
		{name: "Edge: viewer role change", etag: campaign.SheetETag(4, 2, campaign.RolePlayer)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, base, tc.etag)
		})
	}
}

func TestETagMatches(t *testing.T) {
	const etag = `"4.2.gm"`
	tests := []struct {
		name        string
		ifNoneMatch string
		want        bool
	}{
		{name: "Edge: empty header never matches", ifNoneMatch: "", want: false},
		{name: "Edge: whitespace only never matches", ifNoneMatch: "   ", want: false},
		{name: "Edge: stale tag", ifNoneMatch: `"4.1.gm"`, want: false},
		{name: "Edge: same numbers with other role", ifNoneMatch: `"4.2.player"`, want: false},
		{name: "Edge: unquoted tag does not match", ifNoneMatch: `4.2.gm`, want: false},
		{name: "Happy path: exact match", ifNoneMatch: `"4.2.gm"`, want: true},
		{name: "Happy path: weak validator matches", ifNoneMatch: `W/"4.2.gm"`, want: true},
		{name: "Happy path: match inside a list", ifNoneMatch: `"1.1.gm", "4.2.gm" , "9.9.gm"`, want: true},
		{name: "Happy path: wildcard matches an existing resource", ifNoneMatch: `*`, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, campaign.ETagMatches(tc.ifNoneMatch, etag))
		})
	}
}
