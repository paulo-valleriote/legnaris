package template

import (
	"strings"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const textLengthSheetKey = "attacks.attack1.notes"

func textLengthDAG(t *testing.T, noteMaxLength *int) *DAG {
	t.Helper()

	fields := FieldGroup{
		"attacks": {Children: FieldGroup{
			"attack1": {Children: FieldGroup{
				"name": {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}},
				"notes": {
					Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true, MaxLength: noteMaxLength},
				},
				"bonus": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true}},
			}},
		}},
	}

	dag, err := BuildDAG("text_length_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

func emptyTextLengthSheet() map[string]any {
	return map[string]any{"attacks": map[string]any{"attack1": map[string]any{}}}
}

func TestDAG_TextMaxLength(t *testing.T) {
	sixty := intPtr(60)
	atLimit := strings.Repeat("a", 60)

	tests := []struct {
		name     string
		maxLen   *int
		original map[string]any
		updates  map[string]any
		wantErr  bool
		wantLen  int
	}{
		{
			name:     "Happy path: a note of exactly the limit is accepted",
			maxLen:   sixty,
			original: emptyTextLengthSheet(),
			updates:  map[string]any{textLengthSheetKey: atLimit},
		},
		{
			name:     "Happy path: multi-byte runes count as one character each",
			maxLen:   intPtr(2),
			original: emptyTextLengthSheet(),
			updates:  map[string]any{textLengthSheetKey: "🎲🎲"},
		},
		{
			name:     "Edge: a text leaf without a limit accepts any length",
			maxLen:   nil,
			original: emptyTextLengthSheet(),
			updates:  map[string]any{textLengthSheetKey: strings.Repeat("a", 500)},
		},
		{
			name:     "Edge: a note on an otherwise empty attack slot is accepted",
			maxLen:   sixty,
			original: emptyTextLengthSheet(),
			updates:  map[string]any{textLengthSheetKey: "ranged · 150/600"},
		},
		{
			name:   "Edge: a stored attack without the notes key accepts an unrelated update",
			maxLen: sixty,
			original: map[string]any{"attacks": map[string]any{"attack1": map[string]any{
				"name": "Longbow",
			}}},
			updates: map[string]any{"attacks.attack1.bonus": 3.0},
		},
		{
			name:   "Edge: already stored over-length text is untouched by an unrelated update",
			maxLen: sixty,
			original: map[string]any{"attacks": map[string]any{"attack1": map[string]any{
				"notes": strings.Repeat("a", 61),
				"bonus": 0.0,
			}}},
			updates: map[string]any{"attacks.attack1.bonus": 3.0},
		},
		{
			name:     "Error: a note over the limit returns TextTooLongError",
			maxLen:   sixty,
			original: emptyTextLengthSheet(),
			updates:  map[string]any{textLengthSheetKey: strings.Repeat("a", 61)},
			wantErr:  true,
			wantLen:  61,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag := textLengthDAG(t, tc.maxLen)

			_, _, err := dag.NormalizeAndRecalculate(tc.original, tc.updates)

			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			var tooLong *TextTooLongError
			require.ErrorAs(t, err, &tooLong)
			assert.Equal(t, textLengthSheetKey, tooLong.Key)
			assert.Equal(t, tc.wantLen, tooLong.Length)
			assert.Equal(t, 60, tooLong.Max)
		})
	}
}
