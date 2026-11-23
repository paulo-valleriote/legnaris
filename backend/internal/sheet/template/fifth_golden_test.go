package template

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/require"
)

const fifthGoldenDir = "../../../../testdata/templates"

var updateFifthGolden = flag.Bool("update-fifth-golden", false, "rewrite the 5E Template golden DTOs from the current registry")

type fifthGoldenEnvelope struct {
	Data TemplateDTO `json:"data"`
}

func TestFifthTemplateGoldens(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine(), WithDraftsAllowed(true))
	require.NoError(t, err)

	tests := []struct {
		name string
		id   string
	}{
		{name: "Happy path: the 5E Character DTO matches its golden file", id: fifthCharacterID},
		{name: "Happy path: the 5E Stat Block DTO matches its golden file", id: fifthStatBlockID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, ok := registry.FetchByID(tc.id)
			require.True(t, ok)

			encoded, err := json.Marshal(fifthGoldenEnvelope{Data: tmpl.ToDTO()})
			require.NoError(t, err)
			var current bytes.Buffer
			require.NoError(t, json.Compact(&current, encoded))

			path := filepath.Join(fifthGoldenDir, tc.id+".json")
			if *updateFifthGolden {
				require.NoError(t, os.MkdirAll(fifthGoldenDir, 0o755))
				var pretty bytes.Buffer
				require.NoError(t, json.Indent(&pretty, current.Bytes(), "", "  "))
				pretty.WriteByte('\n')
				require.NoError(t, os.WriteFile(path, pretty.Bytes(), 0o644))
			}

			raw, err := os.ReadFile(filepath.FromSlash(path))
			require.NoError(t, err)
			var golden bytes.Buffer
			require.NoError(t, json.Compact(&golden, raw))
			require.Equal(t, golden.String(), current.String(),
				"the %s DTO drifted from %s; run go test -run TestFifthTemplateGoldens -update-fifth-golden and review the diff", tc.id, tc.id)
		})
	}
}

func TestFifthTemplateGoldens_Contract(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine(), WithDraftsAllowed(true))
	require.NoError(t, err)

	t.Run("Edge: the 5E Character golden names the character kind", func(t *testing.T) {
		tmpl, ok := registry.FetchByID(fifthCharacterID)
		require.True(t, ok)
		require.Len(t, tmpl.Kinds, 1)
		require.Equal(t, "character", string(tmpl.Kinds[0]))
	})

	t.Run("Edge: both 5E Templates resolve the fifth game system", func(t *testing.T) {
		for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
			tmpl, ok := registry.FetchByID(id)
			require.True(t, ok)
			require.Equal(t, GameSystemFifth, tmpl.GameSystemID)
			require.Equal(t, StateDraft, tmpl.State)
		}
	})
}
