package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFieldSchema_SeedValue covers the type-driven seeding contract the
// sheet service uses on Create: text must start empty (Default is a
// float64 and cannot express a string), booleans start false, and every
// other type starts at its numeric default.
func TestFieldSchema_SeedValue(t *testing.T) {
	tests := []struct {
		name   string
		schema FieldSchema
		want   any
	}{
		{
			name:   "Happy path: text field seeds empty regardless of Default",
			schema: FieldSchema{Type: fieldTypeText, Default: 42},
			want:   "",
		},
		{
			name:   "Happy path: number field seeds its default",
			schema: FieldSchema{Type: fieldTypeNumber, Default: 7.5},
			want:   7.5,
		},
		{
			name:   "Edge: number without explicit default seeds zero",
			schema: FieldSchema{Type: fieldTypeNumber},
			want:   0.0,
		},
		{
			name:   "Edge: bool seeds false regardless of the numeric default",
			schema: FieldSchema{Type: fieldTypeBool, Default: 1},
			want:   false,
		},
		{
			name:   "Edge: unknown type falls back to the numeric default",
			schema: FieldSchema{Type: "date", Default: 3},
			want:   3.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.schema.SeedValue())
		})
	}
}
