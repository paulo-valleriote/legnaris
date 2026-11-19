package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var requiredPrefixes = []string{"Happy path:", "Edge:", "Error:"}

const callTablePrefix = "\x00call:"

type violation struct {
	position token.Position
	message  string
}

type requirement struct {
	key      string
	elements []*ast.CompositeLit
	field    string
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	code, err := run(root, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testnames: "+err.Error())
		os.Exit(2)
	}
	os.Exit(code)
}

func run(root string, out io.Writer) (int, error) {
	violations, err := checkTree(root)
	if err != nil {
		return 2, err
	}
	for _, v := range violations {
		if _, err := fmt.Fprintf(out, "%s:%d: %s\n", v.position.Filename, v.position.Line, v.message); err != nil {
			return 2, err
		}
	}
	if len(violations) > 0 {
		return 1, nil
	}
	return 0, nil
}

func checkTree(root string) ([]violation, error) {
	var violations []violation
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		fileViolations, err := checkFile(path)
		if err != nil {
			return err
		}
		violations = append(violations, fileViolations...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

func skipDir(name string) bool {
	switch name {
	case "vendor", "testdata", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func checkFile(path string) ([]violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	fileTables := map[string][]*ast.CompositeLit{}
	helperTables := map[string][]*ast.CompositeLit{}
	var funcs []*ast.FuncDecl
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range value.Names {
					fileTables[name.Name] = append(fileTables[name.Name], valueElements(value.Values)...)
				}
			}
		case *ast.FuncDecl:
			if d.Body == nil || d.Name == nil {
				continue
			}
			funcs = append(funcs, d)
			helperTables[d.Name.Name] = returnedElements(d.Body)
		}
	}
	var violations []violation
	for _, fn := range funcs {
		violations = append(violations, checkFunc(fset, fn.Body, fileTables, helperTables)...)
	}
	seen := map[string]bool{}
	unique := violations[:0]
	for _, v := range violations {
		key := fmt.Sprintf("%s:%d:%s", v.position.Filename, v.position.Line, v.message)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, v)
	}
	return unique, nil
}

func checkFunc(fset *token.FileSet, body *ast.BlockStmt, fileTables, helperTables map[string][]*ast.CompositeLit) []violation {
	requirements := collectRequirements(body)
	if len(requirements) == 0 {
		return nil
	}
	localTables := collectTables(body)
	var violations []violation
	for _, req := range requirements {
		elements := req.elements
		if elements == nil {
			elements = lookupTable(req.key, localTables, fileTables, helperTables)
		}
		for _, element := range elements {
			value := fieldValue(element, req.field)
			literal, ok := value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			name, err := strconv.Unquote(literal.Value)
			if err != nil {
				continue
			}
			if hasRequiredPrefix(name) {
				continue
			}
			violations = append(violations, violation{
				position: fset.Position(literal.Pos()),
				message:  fmt.Sprintf("test case name %q must start with %q, %q or %q", name, requiredPrefixes[0], requiredPrefixes[1], requiredPrefixes[2]),
			})
		}
	}
	return violations
}

func lookupTable(key string, local, file, helpers map[string][]*ast.CompositeLit) []*ast.CompositeLit {
	if name, ok := strings.CutPrefix(key, callTablePrefix); ok {
		return helpers[name]
	}
	if elements, ok := local[key]; ok {
		return elements
	}
	return file[key]
}

func hasRequiredPrefix(name string) bool {
	for _, prefix := range requiredPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func collectRequirements(body *ast.BlockStmt) []requirement {
	seen := map[string]bool{}
	var requirements []requirement
	ast.Inspect(body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		key, elements := rangeTable(stmt.X)
		if key == "" && elements == nil {
			return true
		}
		loopVars := rangeVarNames(stmt)
		ast.Inspect(stmt.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isRunCall(call) || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Args[0].(*ast.SelectorExpr)
			if !ok {
				return true
			}
			target, ok := selector.X.(*ast.Ident)
			if !ok || !loopVars[target.Name] {
				return true
			}
			req := requirement{key: key, field: selector.Sel.Name}
			if elements != nil {
				req.key = positionalKey(elements[0].Pos())
				req.elements = elements
			}
			dedupe := req.key + "\x00" + req.field
			if !seen[dedupe] {
				seen[dedupe] = true
				requirements = append(requirements, req)
			}
			return true
		})
		return true
	})
	return requirements
}

func rangeTable(expr ast.Expr) (string, []*ast.CompositeLit) {
	if expr == nil {
		return "", nil
	}
	switch e := unwrapParen(expr).(type) {
	case *ast.CompositeLit:
		return "", literalElements(e)
	case *ast.CallExpr:
		ident, ok := e.Fun.(*ast.Ident)
		if !ok {
			return "", nil
		}
		return callTablePrefix + ident.Name, nil
	}
	return exprKey(expr), nil
}

func rangeVarNames(stmt *ast.RangeStmt) map[string]bool {
	names := map[string]bool{}
	for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
		if ident, ok := expr.(*ast.Ident); ok && ident.Name != "_" {
			names[ident.Name] = true
		}
	}
	return names
}

func isRunCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Run"
}

func positionalKey(pos token.Pos) string {
	return fmt.Sprintf("\x00literal:%d", pos)
}

func exprKey(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		prefix := exprKey(e.X)
		if prefix == "" {
			return ""
		}
		return prefix + "." + e.Sel.Name
	case *ast.ParenExpr:
		return exprKey(e.X)
	}
	return ""
}

func collectTables(body *ast.BlockStmt) map[string][]*ast.CompositeLit {
	tables := map[string][]*ast.CompositeLit{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
				return true
			}
			name, ok := n.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			tables[name.Name] = append(tables[name.Name], tableElements(n.Rhs[0])...)
		case *ast.ValueSpec:
			if len(n.Names) == 0 {
				return true
			}
			elements := valueElements(n.Values)
			for _, name := range n.Names {
				tables[name.Name] = append(tables[name.Name], elements...)
			}
		}
		return true
	})
	return tables
}

func returnedElements(body *ast.BlockStmt) []*ast.CompositeLit {
	var elements []*ast.CompositeLit
	ast.Inspect(body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, result := range stmt.Results {
			elements = append(elements, tableElements(result)...)
		}
		return true
	})
	return elements
}

func valueElements(values []ast.Expr) []*ast.CompositeLit {
	var elements []*ast.CompositeLit
	for _, value := range values {
		elements = append(elements, tableElements(value)...)
	}
	return elements
}

func tableElements(expr ast.Expr) []*ast.CompositeLit {
	switch e := unwrapParen(expr).(type) {
	case *ast.CallExpr:
		return appendElements(e)
	case *ast.CompositeLit:
		return literalElements(e)
	}
	return nil
}

func unwrapParen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func appendElements(call *ast.CallExpr) []*ast.CompositeLit {
	fun, ok := call.Fun.(*ast.Ident)
	if !ok || fun.Name != "append" || len(call.Args) < 2 {
		return nil
	}
	if call.Ellipsis != token.NoPos {
		return tableElements(call.Args[len(call.Args)-1])
	}
	var elements []*ast.CompositeLit
	for _, arg := range call.Args[1:] {
		if element := derefComposite(arg); element != nil {
			elements = append(elements, element)
			continue
		}
		elements = append(elements, tableElements(arg)...)
	}
	return elements
}

func literalElements(literal *ast.CompositeLit) []*ast.CompositeLit {
	switch literal.Type.(type) {
	case *ast.ArrayType:
		return compositeElements(literal.Elts)
	case *ast.MapType:
		var elements []*ast.CompositeLit
		for _, elt := range literal.Elts {
			pair, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if value := derefComposite(pair.Value); value != nil {
				elements = append(elements, value)
			}
		}
		return elements
	}
	return nil
}

func compositeElements(exprs []ast.Expr) []*ast.CompositeLit {
	var elements []*ast.CompositeLit
	for _, expr := range exprs {
		if value := derefComposite(expr); value != nil {
			elements = append(elements, value)
		}
	}
	return elements
}

func derefComposite(expr ast.Expr) *ast.CompositeLit {
	switch e := unwrapParen(expr).(type) {
	case *ast.CompositeLit:
		return e
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			if value, ok := unwrapParen(e.X).(*ast.CompositeLit); ok {
				return value
			}
		}
	}
	return nil
}

func fieldValue(literal *ast.CompositeLit, field string) ast.Expr {
	for _, elt := range literal.Elts {
		pair, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != field {
			continue
		}
		return unwrapParen(pair.Value)
	}
	return nil
}
