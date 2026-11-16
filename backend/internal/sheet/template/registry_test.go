package template

import (
	"legnaris/backend/internal/engine"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tamperTemplate(tmpl *Template) {
	tmpl.Name = "tampered"
	tmpl.Fields["tampered"] = FieldNode{}
	tamperFirstLeaf(tmpl.Fields)
}

func tamperFirstLeaf(group FieldGroup) {
	for name, node := range group {
		if node.Schema != nil {
			node.Schema.Type = "tampered"
			node.Schema.Formula = "tampered"
			node.Schema.IsEditable = !node.Schema.IsEditable
			node.Schema.DependsOn = append(node.Schema.DependsOn, []string{"tampered"})
			group[name+"_tampered"] = FieldNode{}
			return
		}
		if node.Children != nil {
			tamperFirstLeaf(node.Children)
			return
		}
	}
}

func TestCloneFieldNode_DeepCopiesTableMetadata(t *testing.T) {
	source := FieldNode{
		Children: FieldGroup{
			"bonus": {Schema: &FieldSchema{Type: fieldTypeNumber}},
		},
		Label:   "Attack 1",
		Columns: []string{"name", "damage", "bonus"},
	}

	cloned := cloneFieldNode(source)

	assert.Equal(t, source.Label, cloned.Label)
	assert.Equal(t, source.Columns, cloned.Columns)

	cloned.Label = "tampered"
	cloned.Columns[0] = "tampered"
	assert.Equal(t, "Attack 1", source.Label)
	assert.Equal(t, "name", source.Columns[0],
		"the cloned node aliases the source column list")
}

func TestRegistry_ListReturnsDeepCopies(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	list := registry.List()
	require.NotEmpty(t, list)

	for i := range list {
		tamperTemplate(&list[i])
	}

	assert.Equal(t, DefaultTemplates(), registry.List(),
		"mutating templates returned by List changed the registry")
}

func TestRegistry_FetchByIDReturnsDeepCopy(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	first, ok := registry.FetchByID(fantasyV1Template.ID)
	require.True(t, ok)

	tamperTemplate(first)

	second, ok := registry.FetchByID(fantasyV1Template.ID)
	require.True(t, ok)
	assert.Equal(t, fantasyV1Template, *second,
		"mutating the template returned by FetchByID changed the registry")
}

func TestRegistry_IsIsolatedFromDefaultTemplatesCatalog(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	templates := DefaultTemplates()
	entry := &templates[0]
	dex := entry.Fields["stats"].Children["dexterity"].Schema
	require.NotNil(t, dex)

	savedName, savedDefault := entry.Name, entry.Default
	savedType, savedEditable := dex.Type, dex.IsEditable
	t.Cleanup(func() {
		entry.Name, entry.Default = savedName, savedDefault
		dex.Type, dex.IsEditable = savedType, savedEditable
		delete(entry.Fields, "tampered")
	})

	entry.Name = "tampered"
	entry.Default = !entry.Default
	entry.Fields["tampered"] = FieldNode{}
	dex.Type = "tampered"
	dex.IsEditable = !dex.IsEditable

	fetched, ok := registry.FetchByID(entry.ID)
	require.True(t, ok)
	assert.Equal(t, savedName, fetched.Name)
	assert.Equal(t, savedDefault, fetched.Default)
	assert.NotContains(t, fetched.Fields, "tampered")
	assert.Equal(t, savedType, fetched.Fields["stats"].Children["dexterity"].Schema.Type)

	listed := registry.List()
	require.NotEmpty(t, listed)
	assert.Equal(t, savedName, listed[0].Name)
	assert.NotContains(t, listed[0].Fields, "tampered")
}

func TestRegistry_DAGReturnsSingleSharedGraph(t *testing.T) {
	registry, err := BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	first, ok := registry.DAG(fantasyV1Template.ID)
	require.True(t, ok)
	second, ok := registry.DAG(fantasyV1Template.ID)
	require.True(t, ok)
	assert.Same(t, first, second, "each lookup must share one immutable DAG")

	_, ok = registry.DAG("does_not_exist")
	assert.False(t, ok)

	if method, found := reflect.TypeOf(registry).MethodByName("DAGs"); found {
		t.Errorf("Registry.DAGs hands out the internal map; resolve DAGs through DAG(id) "+
			"instead (LGN-54), found %s", method.Type)
	}
}
