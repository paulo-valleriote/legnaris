package generation_test

import (
	"os"
	"path/filepath"
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFantasyV1SchemaSnapshot(t *testing.T) {
	tmpl := fantasyV1Fixture(t)

	tests := []struct {
		name string
		kind plan.SheetKind
	}{
		{name: "Happy path: the frozen character schema is byte-identical", kind: plan.KindCharacter},
		{name: "Happy path: the frozen NPC schema is byte-identical", kind: plan.KindNPC},
		{name: "Happy path: the frozen enemy schema is byte-identical", kind: plan.KindEnemy},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := mustSchema(t, tmpl, tc.kind)

			path := filepath.Join("testdata", "fantasy_v1_"+string(tc.kind)+"_schema.golden.json")
			want, err := os.ReadFile(path)
			require.NoError(t, err)

			assert.Equal(t, string(want), string(raw))
		})
	}
}
