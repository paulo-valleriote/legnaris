package mockscheck

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var ErrMissingGeneratedHeader = errors.New("mock file is missing the \"// Code generated ... DO NOT EDIT.\" header")

var ErrMockOutsideMocks = errors.New("mock type declared outside a mocks/ directory")

var ErrMockeryUnpinned = errors.New("mockery is not pinned by the go.mod tool directive")

var mockTypeName = regexp.MustCompile(`^Mock\w+$`)

var pinnedVersion = regexp.MustCompile(`^\s*github\.com/vektra/mockery/v2\s+v[0-9]+\.[0-9]+\.[0-9]+(\s+//.*)?$`)

const mockeryModule = "github.com/vektra/mockery/v2"

func IsMocksPath(p string) bool {
	segments := strings.Split(p, "/")
	for _, segment := range segments[:len(segments)-1] {
		if segment == "mocks" {
			return true
		}
	}
	return false
}

func Issues(p string, content []byte) []string {
	if IsMocksPath(p) {
		if hasGeneratedHeader(content) {
			return nil
		}
		return []string{fmt.Sprintf("%s: %v", p, ErrMissingGeneratedHeader)}
	}
	var issues []string
	for _, name := range mockStructs(content) {
		issues = append(issues, fmt.Sprintf("%s: %v: type %s struct", p, ErrMockOutsideMocks, name))
	}
	return issues
}

func MockeryPinned(gomod []byte) error {
	if !declaresTool(gomod) {
		return fmt.Errorf("%w: go.mod has no \"tool %s\" directive", ErrMockeryUnpinned, mockeryModule)
	}
	for _, line := range strings.Split(string(gomod), "\n") {
		if pinnedVersion.MatchString(line) {
			return nil
		}
	}
	return fmt.Errorf("%w: go.mod does not require an exact %s version", ErrMockeryUnpinned, mockeryModule)
}

func hasGeneratedHeader(content []byte) bool {
	header := packagePreamble(content)
	return bytes.Contains(header, []byte("Code generated")) &&
		bytes.Contains(header, []byte("DO NOT EDIT."))
}

func packagePreamble(content []byte) []byte {
	offset := 0
	for offset < len(content) {
		end := bytes.IndexByte(content[offset:], '\n')
		line := content[offset:]
		if end >= 0 {
			line = content[offset : offset+end]
		}
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("package ")) {
			return content[:offset]
		}
		if end < 0 {
			break
		}
		offset += end + 1
	}
	return content
}

func declaresTool(gomod []byte) bool {
	inBlock := false
	for _, line := range strings.Split(string(gomod), "\n") {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			switch trimmed {
			case "tool " + mockeryModule:
				return true
			case "tool (":
				inBlock = true
			}
			continue
		}
		switch trimmed {
		case ")":
			inBlock = false
		case mockeryModule:
			return true
		}
	}
	return false
}

func mockStructs(content []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", content, parser.SkipObjectResolution)
	if err != nil || file == nil {
		return nil
	}
	var names []string
	for _, decl := range file.Decls {
		generic, ok := decl.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, spec := range generic.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); !ok {
				continue
			}
			if mockTypeName.MatchString(typeSpec.Name.Name) {
				names = append(names, typeSpec.Name.Name)
			}
		}
	}
	return names
}
