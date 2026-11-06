package dice_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/dice"
)

func vectorSchema(t *testing.T, set vectorSet) dice.MapSchema {
	t.Helper()
	schema := dice.MapSchema{}
	for path, kind := range set.Fields {
		switch kind {
		case "number":
			schema[path] = dice.FieldNumber
		case "text":
			schema[path] = dice.FieldText
		case "bool":
			schema[path] = dice.FieldBool
		default:
			require.Failf(t, "unknown field kind in vectors", "%s: %s", path, kind)
		}
	}
	return schema
}

func TestFitsRow_Vectors(t *testing.T) {
	set := loadVectors(t)
	schema := vectorSchema(t, set)
	require.NotEmpty(t, set.Fit)

	for _, vec := range set.Fit {
		t.Run(prefixFor(vec.Fits)+vec.Name, func(t *testing.T) {
			expr, err := dice.Parse(vec.Expr)
			require.NoError(t, err)

			err = expr.FitsRow(schema, vec.Row)
			if vec.Fits {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, vec.Class, classOf(t, err))
		})
	}
}

func TestValidateRow_Vectors(t *testing.T) {
	set := loadVectors(t)
	schema := vectorSchema(t, set)

	for _, vec := range set.Fit {
		t.Run(prefixFor(vec.Fits)+vec.Name, func(t *testing.T) {
			expr, err := dice.ValidateRow(schema, vec.Row, vec.Expr)
			if vec.Fits {
				require.NoError(t, err)
				assert.NotEmpty(t, expr.Terms())
				return
			}
			require.Error(t, err)
			assert.Equal(t, vec.Class, classOf(t, err))
		})
	}

	for _, vec := range set.Invalid {
		t.Run("Error: syntax first: "+vec.Name, func(t *testing.T) {
			_, err := dice.ValidateRow(schema, "combat.initiative", vec.Expr)
			require.Error(t, err)
			assert.Equal(t, vec.Class, classOf(t, err))
		})
	}
}

func TestValidateSection_Vectors(t *testing.T) {
	set := loadVectors(t)
	schema := vectorSchema(t, set)
	require.NotEmpty(t, set.Sections)

	for _, vec := range set.Sections {
		t.Run(prefixFor(vec.Fits)+vec.Name, func(t *testing.T) {
			_, err := dice.ValidateSection(schema, vec.Rows, vec.Expr)
			if vec.Fits {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, vec.Class, classOf(t, err))
		})
	}
}

func TestFitsRow_Edges(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		schema dice.Schema
		want   error
	}{
		{"Edge: nil schema finds no Field", "1d20 + stats.dexterity", nil, dice.ErrUnknownField},
		{"Edge: nil schema and no references fits", "1d20 + 3", nil, nil},
		{"Edge: nil map schema finds no Field", "@", dice.MapSchema(nil), dice.ErrUnknownField},
		{"Edge: empty schema and no references fits", "1d20", dice.MapSchema{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := dice.Parse(tc.expr)
			require.NoError(t, err)

			err = expr.FitsRow(tc.schema, "combat.initiative")

			if tc.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.want)
		})
	}

	t.Run("Edge: zero expression fits everything", func(t *testing.T) {
		var zero dice.Expression
		assert.NoError(t, zero.FitsRow(dice.MapSchema{}, "anything"))
	})
}

func TestMapSchema_FieldKind(t *testing.T) {
	schema := dice.MapSchema{"a.b": dice.FieldNumber}
	tests := []struct {
		name   string
		path   string
		wantOK bool
		want   dice.FieldKind
	}{
		{"Happy path: known leaf", "a.b", true, dice.FieldNumber},
		{"Edge: a group is not a Field", "a", false, 0},
		{"Edge: empty path", "", false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, ok := schema.FieldKind(tc.path)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, kind)
		})
	}
}

func TestFitsRow_IsConcurrencySafe(t *testing.T) {
	schema := dice.MapSchema{"skills.acrobatics.total": dice.FieldNumber}
	expr, err := dice.Parse("1d20 + @.total")
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			assert.NoError(t, expr.FitsRow(schema, "skills.acrobatics"))
			assert.ErrorIs(t, expr.FitsRow(schema, "combat.initiative"), dice.ErrUnknownField)
		})
	}
	wg.Wait()
}

func prefixFor(fits bool) string {
	if fits {
		return "Happy path: "
	}
	return "Error: "
}
