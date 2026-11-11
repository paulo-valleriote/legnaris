package template

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonKeyOrder returns the keys of the JSON object at the given path in
// document order, which a map decode would lose.
func jsonKeyOrder(t *testing.T, raw []byte, path ...string) []string {
	t.Helper()
	for _, key := range path {
		var object map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &object))
		raw = object[key]
		require.NotNil(t, raw, "missing key %q", key)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	_, err := decoder.Token()
	require.NoError(t, err)
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		require.NoError(t, err)
		keys = append(keys, token.(string))
		var skip json.RawMessage
		require.NoError(t, decoder.Decode(&skip))
	}
	return keys
}

func TestTemplateDTO_MarshalJSON_NestedOrder(t *testing.T) {
	leaf := numField("x", 0)
	tests := []struct {
		name  string
		order []string
		want  []string
	}{
		{name: "Edge: no order falls back to alphabetical", order: nil, want: []string{"a", "b", "c"}},
		{name: "Edge: partial order puts listed keys first and the rest alphabetically", order: []string{"c"}, want: []string{"c", "a", "b"}},
		{name: "Edge: unknown and duplicate keys are ignored", order: []string{"zzz", "b", "b"}, want: []string{"b", "a", "c"}},
		{name: "Happy path: full order is emitted as declared", order: []string{"c", "a", "b"}, want: []string{"c", "a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := Template{
				ID: "t",
				Fields: FieldGroup{"sec": {
					Children: FieldGroup{"a": leaf, "b": leaf, "c": leaf},
					Order:    tc.order,
				}},
			}
			raw, err := json.Marshal(tmpl.ToDTO())
			require.NoError(t, err)
			assert.Equal(t, tc.want, jsonKeyOrder(t, raw, "fields", "sec", "children"))
		})
	}
}

func TestFantasyV1_ServesConventionalOrder(t *testing.T) {
	raw, err := json.Marshal(fantasyV1Template.ToDTO())
	require.NoError(t, err)

	assert.Equal(t, "characterName", jsonKeyOrder(t, raw, "fields", "identity", "children")[0])
	assert.Equal(t,
		[]string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"},
		jsonKeyOrder(t, raw, "fields", "stats", "children"))
	assert.Equal(t,
		[]string{"strengthMod", "dexterityMod", "constitutionMod", "intelligenceMod", "wisdomMod", "charismaMod"},
		jsonKeyOrder(t, raw, "fields", "modifiers", "children"))
	assert.Equal(t, []string{"bonus", "total"}, jsonKeyOrder(t, raw, "fields", "skills", "children", "athletics", "children"))
}

func TestTemplateOrder_DoesNotAliasAfterClone(t *testing.T) {
	cloned := cloneTemplate(fantasyV1Template)
	require.Equal(t, fantasyV1Template.Fields[fieldNameStats].Order, cloned.Fields[fieldNameStats].Order)

	cloned.Fields[fieldNameStats].Order[0] = "mutated"
	assert.Equal(t, abilityStrength, fantasyV1Template.Fields[fieldNameStats].Order[0])
}
