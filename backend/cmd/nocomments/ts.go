package main

import (
	"regexp"
	"strings"
)

type tsMode int

const (
	modeCode tsMode = iota
	modeTemplate
	modeTag
	modeChildren
)

type tsContext struct {
	mode    tsMode
	braces  int
	bounded bool
}

type tsScanner struct {
	path         string
	src          []byte
	tsx          bool
	i            int
	stack        []tsContext
	regexAllowed bool
	violations   []Violation
}

func scanTS(path string, src []byte) []Violation {
	if isGenerated(src) {
		return nil
	}
	lexer := &tsScanner{
		path:         path,
		src:          src,
		tsx:          strings.HasSuffix(strings.ToLower(path), ".tsx"),
		stack:        []tsContext{{mode: modeCode}},
		regexAllowed: true,
	}
	lexer.run()
	return lexer.violations
}

func (s *tsScanner) run() {
	for s.i < len(s.src) {
		switch s.top().mode {
		case modeCode:
			s.scanCode()
		case modeTemplate:
			s.scanTemplate()
		case modeTag:
			s.scanTag()
		case modeChildren:
			s.scanChildren()
		}
	}
}

func (s *tsScanner) top() tsContext {
	return s.stack[len(s.stack)-1]
}

func (s *tsScanner) push(mode tsMode) {
	s.stack = append(s.stack, tsContext{mode: mode})
}

func (s *tsScanner) pushCode() {
	s.stack = append(s.stack, tsContext{mode: modeCode, braces: 1, bounded: true})
	s.regexAllowed = true
}

func (s *tsScanner) pop() {
	if len(s.stack) > 1 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

func (s *tsScanner) peek(offset int) byte {
	index := s.i + offset
	if index < 0 || index >= len(s.src) {
		return 0
	}
	return s.src[index]
}

func (s *tsScanner) scanCode() {
	c := s.src[s.i]
	if isSpace(c) {
		s.i++
		return
	}
	next := s.peek(1)
	switch {
	case c == '/' && next == '/':
		s.lineComment()
	case c == '/' && next == '*':
		s.blockComment()
	case c == '\'' || c == '"':
		s.quoted(c)
	case c == '`':
		s.i++
		s.push(modeTemplate)
	case c == '/' && s.regexAllowed:
		s.regexLiteral()
	case s.isJSXStart(c):
		s.openJSX()
	case c == '{' || c == '}':
		s.brace(c)
	default:
		s.codeToken(c)
	}
}

func (s *tsScanner) isJSXStart(c byte) bool {
	return s.tsx && c == '<' && s.regexAllowed && s.jsxStarts()
}

func (s *tsScanner) brace(c byte) {
	if c == '{' {
		s.openBrace()
		return
	}
	s.closeBrace()
}

func (s *tsScanner) scanTemplate() {
	c := s.src[s.i]
	switch {
	case c == '\\':
		s.skipEscape()
	case c == '`':
		s.i++
		s.pop()
		s.regexAllowed = false
	case c == '$' && s.peek(1) == '{':
		s.i += 2
		s.pushCode()
	default:
		s.i++
	}
}

func (s *tsScanner) scanTag() {
	c := s.src[s.i]
	if isSpace(c) {
		s.i++
		return
	}
	switch c {
	case '{':
		s.i++
		s.pushCode()
	case '\'', '"':
		s.quoted(c)
	case '/':
		if s.peek(1) == '>' {
			s.i += 2
			s.pop()
			s.regexAllowed = false
			return
		}
		s.i++
	case '>':
		s.i++
		s.pop()
		s.push(modeChildren)
	default:
		s.i++
	}
}

func (s *tsScanner) scanChildren() {
	c := s.src[s.i]
	switch c {
	case '{':
		s.i++
		s.pushCode()
	case '<':
		s.childrenAngle()
	default:
		s.i++
	}
}

func (s *tsScanner) childrenAngle() {
	next := s.peek(1)
	switch {
	case next == '/':
		s.skipClosingTag()
	case next == '>' || isJSXNameStart(next):
		s.i++
		s.push(modeTag)
	default:
		s.i++
	}
}

func (s *tsScanner) skipClosingTag() {
	s.i += 2
	for s.i < len(s.src) && s.src[s.i] != '>' {
		s.i++
	}
	if s.i < len(s.src) {
		s.i++
	}
	s.pop()
	s.regexAllowed = false
}

func (s *tsScanner) openJSX() {
	if s.genericArrowAhead() {
		s.i++
		s.regexAllowed = true
		return
	}
	if s.peek(1) == '>' {
		s.i += 2
		s.push(modeChildren)
		return
	}
	s.i++
	s.push(modeTag)
}

func (s *tsScanner) openBrace() {
	s.i++
	if !s.top().bounded {
		s.regexAllowed = true
		return
	}
	top := &s.stack[len(s.stack)-1]
	top.braces++
	s.regexAllowed = true
}

func (s *tsScanner) closeBrace() {
	s.i++
	if !s.top().bounded {
		s.regexAllowed = false
		return
	}
	top := &s.stack[len(s.stack)-1]
	top.braces--
	if top.braces > 0 {
		return
	}
	s.pop()
	s.regexAllowed = false
}

func (s *tsScanner) quoted(quote byte) {
	s.i++
	for s.i < len(s.src) {
		c := s.src[s.i]
		switch c {
		case '\\':
			s.skipEscape()
		case quote:
			s.i++
			s.regexAllowed = false
			return
		case '\n':
			s.regexAllowed = false
			return
		default:
			s.i++
		}
	}
	s.regexAllowed = false
}

func (s *tsScanner) skipEscape() {
	s.i += 2
	if s.i > len(s.src) {
		s.i = len(s.src)
	}
}

func (s *tsScanner) regexLiteral() {
	s.i++
	inClass := false
	for s.i < len(s.src) {
		c := s.src[s.i]
		if c == '\\' {
			s.skipEscape()
			continue
		}
		if c == '\n' {
			break
		}
		if c == '[' {
			inClass = true
			s.i++
			continue
		}
		if c == ']' {
			inClass = false
			s.i++
			continue
		}
		if c == '/' && !inClass {
			s.i++
			break
		}
		s.i++
	}
	for s.i < len(s.src) && isIdentPart(s.src[s.i]) {
		s.i++
	}
	s.regexAllowed = false
}

func (s *tsScanner) codeToken(c byte) {
	switch {
	case isIdentStart(c):
		s.identifier()
	case c >= '0' && c <= '9':
		s.number()
	case c == '+' || c == '-':
		s.punctuation(c)
	case c == ')' || c == ']' || c == '.':
		s.i++
		s.regexAllowed = false
	default:
		s.i++
		s.regexAllowed = true
	}
}

func (s *tsScanner) identifier() {
	start := s.i
	for s.i < len(s.src) && isIdentPart(s.src[s.i]) {
		s.i++
	}
	s.regexAllowed = expressionKeywords[string(s.src[start:s.i])]
}

func (s *tsScanner) number() {
	for s.i < len(s.src) && (isIdentPart(s.src[s.i]) || s.src[s.i] == '.') {
		s.i++
	}
	s.regexAllowed = false
}

func (s *tsScanner) punctuation(c byte) {
	s.i++
	if s.peek(0) == c {
		s.i++
		s.regexAllowed = false
		return
	}
	s.regexAllowed = true
}

func (s *tsScanner) lineComment() {
	start := s.i
	for s.i < len(s.src) && s.src[s.i] != '\n' {
		s.i++
	}
	s.record(start, string(s.src[start:s.i]))
}

func (s *tsScanner) blockComment() {
	start := s.i
	s.i += 2
	for s.i < len(s.src) && (s.src[s.i] != '*' || s.peek(1) != '/') {
		s.i++
	}
	if s.i < len(s.src) {
		s.i += 2
	}
	s.record(start, string(s.src[start:s.i]))
}

func (s *tsScanner) record(offset int, text string) {
	if allowedTSComment(text) {
		return
	}
	line, column := s.position(offset)
	s.violations = append(s.violations, Violation{
		Path:   s.path,
		Line:   line,
		Column: column,
		Text:   firstLine(text),
	})
}

func (s *tsScanner) position(offset int) (int, int) {
	line, lineStart := 1, 0
	for index := 0; index < offset && index < len(s.src); index++ {
		if s.src[index] == '\n' {
			line++
			lineStart = index + 1
		}
	}
	return line, offset - lineStart + 1
}

func (s *tsScanner) jsxStarts() bool {
	next := s.peek(1)
	if next == '>' {
		return true
	}
	if !isJSXNameStart(next) {
		return false
	}
	return !s.genericArrowAhead()
}

func (s *tsScanner) genericArrowAhead() bool {
	index := s.i + 1
	depth := 0
	hasTypeParameterMarker := false
	for index < len(s.src) {
		c := s.src[index]
		switch {
		case c == '<':
			depth++
		case c == '>':
			if depth == 0 {
				return hasTypeParameterMarker && index+1 < len(s.src) && s.src[index+1] == '('
			}
			depth--
		case c == ',':
			hasTypeParameterMarker = true
		case isIdentStart(c):
			start := index
			for index < len(s.src) && isIdentPart(s.src[index]) {
				index++
			}
			if string(s.src[start:index]) == "extends" {
				hasTypeParameterMarker = true
			}
			continue
		case isTypeParameterChar(c):
		default:
			return false
		}
		index++
	}
	return false
}

func allowedTSComment(text string) bool {
	if strings.HasPrefix(text, "//") {
		return allowedTSLineComment(text)
	}
	return allowedTSBlockComment(text)
}

func allowedTSLineComment(text string) bool {
	if strings.HasPrefix(text, "/// <reference") {
		return true
	}
	return allowedDirective(strings.TrimSpace(strings.TrimPrefix(text, "//")))
}

func allowedTSBlockComment(text string) bool {
	body := strings.TrimPrefix(text, "/*")
	body = strings.TrimSuffix(body, "*/")
	return allowedDirective(strings.TrimSpace(body))
}

func allowedDirective(body string) bool {
	switch {
	case strings.HasPrefix(body, "@ts-expect-error"):
		return true
	case strings.HasPrefix(body, "prettier-ignore"):
		return true
	case strings.HasPrefix(body, "eslint-disable"):
		return !reasonSuffix.MatchString(body)
	default:
		return false
	}
}

var reasonSuffix = regexp.MustCompile(`\s--\s`)

var expressionKeywords = map[string]bool{
	"await":      true,
	"case":       true,
	"delete":     true,
	"do":         true,
	"else":       true,
	"extends":    true,
	"in":         true,
	"instanceof": true,
	"new":        true,
	"of":         true,
	"return":     true,
	"throw":      true,
	"typeof":     true,
	"void":       true,
	"yield":      true,
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isJSXNameStart(c byte) bool {
	return isIdentStart(c)
}

func isTypeParameterChar(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '.', '[', ']', '|', '&', '?', '=', ':', '{', '}',
		'(', ')', '"', '\'':
		return true
	default:
		return false
	}
}
