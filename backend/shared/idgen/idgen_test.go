package idgen_test

import (
	"testing"

	"legnaris/backend/shared/idgen"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew pins the generation contract: New emits a canonical lowercase UUID
// v7 that Validate accepts — the round-trip every server-generated ID goes
// through (LGN-86).
func TestNew(t *testing.T) {
	id, err := idgen.New()

	require.NoError(t, err)
	require.Len(t, id, 36)
	require.NoError(t, idgen.Validate(id))

	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())
	assert.Equal(t, uuid.RFC4122, parsed.Variant())
}

// TestNew_StrictlyIncreasing: the first 8 bytes carry time+sequence, so
// consecutive IDs must sort in creation order — the property listings and
// pagination will rely on to sort by _id.
func TestNew_StrictlyIncreasing(t *testing.T) {
	const count = 1000

	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		id, err := idgen.New()
		require.NoError(t, err)
		ids = append(ids, id)
	}

	for i := 1; i < count; i++ {
		require.Greater(t, ids[i], ids[i-1],
			"ID %d must sort after ID %d (creation order)", i, i-1)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "Happy path: canonical v7 accepted", id: "01900000-0000-7000-8000-000000000001"},
		{name: "Happy path: RFC 4122 variant 9 accepted", id: "01900000-0000-7000-9abc-def012345678"},
		{name: "Edge: empty string rejected", id: "", wantErr: true},
		{name: "Error: garbage string rejected", id: "not-a-uuid", wantErr: true},
		{name: "Error: non-hex character rejected", id: "01900000-0000-7000-800g-000000000001", wantErr: true},
		{name: "Error: too short rejected", id: "01900000-0000-7000-8000-00000000000", wantErr: true},
		{name: "Error: too long rejected", id: "01900000-0000-7000-8000-0000000000000", wantErr: true},
		{name: "Error: UUID v4 (pre-LGN-86 format) rejected", id: "9f2c8a1b-3d4e-4f60-8a1b-2c3d4e5f6071", wantErr: true},
		{name: "Error: UUID v1 rejected", id: "f2c8a1b0-3d4e-1f60-8a1b-2c3d4e5f6071", wantErr: true},
		{name: "Error: nil UUID rejected", id: "00000000-0000-0000-0000-000000000000", wantErr: true},
		{name: "Error: non-RFC 4122 variant rejected", id: "01900000-0000-7000-c000-000000000001", wantErr: true},
		{name: "Edge: uppercase rejected", id: "01900000-0000-7000-8ABC-DEF012345678", wantErr: true},
		{name: "Edge: urn:uuid: encoding rejected", id: "urn:uuid:01900000-0000-7000-8000-000000000001", wantErr: true},
		{name: "Edge: braced encoding rejected", id: "{01900000-0000-7000-8000-000000000001}", wantErr: true},
		{name: "Edge: missing-hyphens encoding rejected", id: "01900000000070008000000000000001", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := idgen.Validate(tc.id)

			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, idgen.ErrInvalidID)
		})
	}
}
