package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"legnaris/backend/internal/dice"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rowIdentityVectorsPath = "../../../../testdata/records/row_identity.json"

type rowIdentityVectors struct {
	Version      int                            `json:"version"`
	Fields       map[string]string              `json:"fields"`
	Lists        map[string]rowIdentityList     `json:"lists"`
	RowKeys      []rowIdentityKeyVector         `json:"rowKeys"`
	Fit          []rowIdentityFitVector         `json:"fit"`
	ListDefaults []rowIdentityListDefaultVector `json:"listDefaults"`
	Sections     []rowIdentitySectionVector     `json:"sections"`
}

type rowIdentityList struct {
	Section string            `json:"section"`
	Shape   map[string]string `json:"shape"`
}

type rowIdentityKeyVector struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Valid   bool   `json:"valid"`
	Section string `json:"section"`
}

type rowIdentityFitVector struct {
	Name  string `json:"name"`
	Row   string `json:"row"`
	Expr  string `json:"expr"`
	Fits  bool   `json:"fits"`
	Class string `json:"class"`
}

type rowIdentityListDefaultVector struct {
	Name  string `json:"name"`
	List  string `json:"list"`
	Expr  string `json:"expr"`
	Fits  bool   `json:"fits"`
	Class string `json:"class"`
}

type rowIdentitySectionVector struct {
	Name    string `json:"name"`
	Section string `json:"section"`
	Expr    string `json:"expr"`
	Fits    bool   `json:"fits"`
	Class   string `json:"class"`
}

func loadRowIdentityVectors(t *testing.T) rowIdentityVectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(rowIdentityVectorsPath))
	require.NoError(t, err)
	var vectors rowIdentityVectors
	require.NoError(t, json.Unmarshal(raw, &vectors))
	require.Equal(t, 1, vectors.Version)
	return vectors
}

func vectorFieldType(t *testing.T, kind string) string {
	t.Helper()
	switch kind {
	case "number":
		return fieldTypeNumber
	case "text":
		return fieldTypeText
	case "bool":
		return fieldTypeBool
	default:
		t.Fatalf("unknown field kind %q", kind)
		return ""
	}
}

func vectorFieldNode(kind string) FieldNode {
	return FieldNode{Schema: &FieldSchema{Type: kind, IsEditable: true}}
}

func insertVectorField(fields FieldGroup, path string, node FieldNode) {
	segments := strings.Split(path, ".")
	cursor := fields
	for _, segment := range segments[:len(segments)-1] {
		next, ok := cursor[segment]
		if !ok {
			next = FieldNode{Children: FieldGroup{}}
		}
		cursor[segment] = next
		cursor = next.Children
	}
	cursor[segments[len(segments)-1]] = node
}

func buildRowIdentityTemplate(t *testing.T, vectors rowIdentityVectors) Template {
	t.Helper()
	fields := FieldGroup{}
	for path, kind := range vectors.Fields {
		insertVectorField(fields, path, vectorFieldNode(vectorFieldType(t, kind)))
	}
	for path, list := range vectors.Lists {
		shape := FieldGroup{}
		for name, kind := range list.Shape {
			shape[name] = vectorFieldNode(vectorFieldType(t, kind))
		}
		insertVectorField(fields, path, FieldNode{List: &ListSchema{
			Label:    path,
			Capacity: 10,
			Shape:    shape,
		}})
	}
	return Template{ID: "row_identity_vectors", Fields: fields}
}

func TestRowIdentityVectors(t *testing.T) {
	vectors := loadRowIdentityVectors(t)
	tmpl := buildRowIdentityTemplate(t, vectors)
	schema := rowDiceSchema(tmpl)

	t.Run("row keys", func(t *testing.T) {
		for _, vector := range vectors.RowKeys {
			t.Run(vector.Name, func(t *testing.T) {
				assert.Equal(t, vector.Valid, IsRowKey(tmpl, vector.Key))
				section, ok := RowSectionKey(tmpl, vector.Key)
				assert.Equal(t, vector.Valid, ok)
				if vector.Valid {
					assert.Equal(t, vector.Section, section)
				}
			})
		}
	})

	t.Run("fit", func(t *testing.T) {
		for _, vector := range vectors.Fit {
			t.Run(vector.Name, func(t *testing.T) {
				_, err := dice.ValidateRow(schema, vector.Row, vector.Expr)
				assertFit(t, vector.Fits, vector.Class, err)
			})
		}
	})

	t.Run("list defaults", func(t *testing.T) {
		for _, vector := range vectors.ListDefaults {
			t.Run(vector.Name, func(t *testing.T) {
				row := RecordRowKey(vector.List, syntheticRecordID)
				_, err := dice.ValidateRow(schema, row, vector.Expr)
				assertFit(t, vector.Fits, vector.Class, err)
			})
		}
	})

	t.Run("sections", func(t *testing.T) {
		rows := sectionRowsForFit(tmpl)
		for _, vector := range vectors.Sections {
			t.Run(vector.Name, func(t *testing.T) {
				_, err := dice.ValidateSection(schema, rows[vector.Section], vector.Expr)
				assertFit(t, vector.Fits, vector.Class, err)
			})
		}
	})
}

func assertFit(t *testing.T, fits bool, wantClass string, err error) {
	t.Helper()
	if fits {
		require.NoError(t, err)
		return
	}
	require.Error(t, err)
	class, ok := dice.ClassOf(err)
	require.True(t, ok)
	assert.Equal(t, wantClass, string(class))
}
