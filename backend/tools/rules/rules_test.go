package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulesFileDeclaresEveryProjectRule(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rules.go", nil, parser.ParseComments)
	require.NoError(t, err)

	reports := map[string]int{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Report" {
				reports[fn.Name.Name]++
			}
			return true
		})
	}

	tests := []struct {
		name string
		rule string
	}{
		{name: "Happy path: apperrBareCause reports a fix", rule: "apperrBareCause"},
		{name: "Happy path: apperrUnwrappedCause reports a fix", rule: "apperrUnwrappedCause"},
		{name: "Happy path: serviceErrorSentinel reports a fix", rule: "serviceErrorSentinel"},
		{name: "Happy path: emptyPrefixGroupMiddleware reports a fix", rule: "emptyPrefixGroupMiddleware"},
	}

	assert.Len(t, reports, len(tests))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Positive(t, reports[tc.rule])
		})
	}
}

func TestRulesFileIsBuildTagged(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rules.go", nil, parser.ParseComments)
	require.NoError(t, err)
	require.NotEmpty(t, file.Comments)
	assert.Equal(t, "//go:build ruleguard", file.Comments[0].List[0].Text)
}
