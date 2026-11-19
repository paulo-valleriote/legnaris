package apperr

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const codesArtifactPath = "../../../testdata/apperr/codes.json"

type codesArtifact struct {
	Version int          `json:"version"`
	Codes   []codesEntry `json:"codes"`
}

type codesEntry struct {
	Code   string `json:"code"`
	Status int    `json:"status"`
}

func declaredCodes(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)

	seen := make(map[string]struct{})
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		{
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					ident, ok := value.Type.(*ast.Ident)
					if !ok || ident.Name != "Code" {
						continue
					}
					require.Len(t, value.Values, len(value.Names), "every declared Code needs a literal value")
					for i := range value.Names {
						literal, ok := value.Values[i].(*ast.BasicLit)
						require.True(t, ok, "a code value must be a literal")
						require.Equal(t, token.STRING, literal.Kind, "a code value must be a string literal")
						code, err := strconv.Unquote(literal.Value)
						require.NoError(t, err)
						seen[code] = struct{}{}
					}
				}
			}
		}
	}

	require.NotEmpty(t, seen, "the package must declare at least one Code")
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func TestDeclaredCodesMapToExplicitStatus(t *testing.T) {
	codes := declaredCodes(t)

	t.Run("Happy path: every declared code maps to an explicit non-fallback status", func(t *testing.T) {
		for _, code := range codes {
			t.Run(code, func(t *testing.T) {
				err := NewError(Code(code), "probe", nil)

				assert.NotZero(t, err.httpStatus, "%s must be mapped in NewError, not fall back to 500", code)
				assert.GreaterOrEqual(t, err.httpStatus, 400)
				assert.Less(t, err.httpStatus, 600)

				if Code(code) == CodeInternal {
					assert.Equal(t, 500, err.httpStatus, "INTERNAL_ERROR is the one code whose mapped status is 500")
					return
				}

				assert.NotEqual(t, 500, err.httpStatus, "%s must not fall back to 500", code)
				assert.Equal(t, err.httpStatus, err.HttpStatus())
			})
		}
	})
}

func TestCodesArtifactIsCurrent(t *testing.T) {
	codes := declaredCodes(t)
	artifact := codesArtifact{Version: 1, Codes: make([]codesEntry, 0, len(codes))}
	for _, code := range codes {
		artifact.Codes = append(artifact.Codes, codesEntry{
			Code:   code,
			Status: NewError(Code(code), "probe", nil).httpStatus,
		})
	}

	encoded, err := json.MarshalIndent(artifact, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')

	if os.Getenv("UPDATE") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(codesArtifactPath), 0o755))
		require.NoError(t, os.WriteFile(codesArtifactPath, encoded, 0o644))
	}

	stored, err := os.ReadFile(codesArtifactPath)
	require.NoError(t, err, "regenerate with UPDATE=1 or `make apperr-codes`")
	require.Equal(t, string(encoded), string(stored),
		"testdata/apperr/codes.json is out of date; regenerate with `UPDATE=1 go test ./backend/shared/apperr -run TestCodesArtifactIsCurrent -count=1` or `make apperr-codes`")
}
