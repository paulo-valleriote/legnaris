package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"regexp"
	"strings"
)

type Violation struct {
	Path   string
	Line   int
	Column int
	Text   string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d:%d: %s", v.Path, v.Line, v.Column, v.Text)
}

var generatedHeader = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.`)

func isGenerated(src []byte) bool {
	return generatedHeader.Match(src)
}

func scanGo(path string, src []byte) []Violation {
	if isGenerated(src) {
		return nil
	}
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))
	var lexer scanner.Scanner
	lexer.Init(file, src, nil, scanner.ScanComments)
	var violations []Violation
	for {
		pos, tok, lit := lexer.Scan()
		if tok == token.EOF {
			return violations
		}
		if tok != token.COMMENT || allowedGoComment(lit) {
			continue
		}
		position := fset.Position(pos)
		violations = append(violations, Violation{
			Path:   path,
			Line:   position.Line,
			Column: position.Column,
			Text:   firstLine(lit),
		})
	}
}

func allowedGoComment(text string) bool {
	return strings.HasPrefix(text, "//go:") || strings.HasPrefix(text, "//nolint")
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}
	return strings.TrimSpace(text)
}
