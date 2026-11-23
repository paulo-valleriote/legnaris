package dice

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxLength      = 64
	MaxTerms       = 8
	MinDice        = 1
	MaxDice        = 20
	MinFaces       = 1
	MaxFaces       = 1000
	MaxWholeNumber = 9999
)

type Kind int

const (
	KindDice Kind = iota + 1
	KindNumber
	KindField
)

type Reference struct {
	Path     string
	Relative bool
	Column   string
}

type Term struct {
	Kind     Kind
	Sign     int
	Count    int
	Faces    int
	Value    int
	Path     string
	Relative bool
	Column   string
	CountRef *Reference
	FacesRef *Reference
}

type Expression struct {
	terms   []Term
	minimum *int
}

type partResult struct {
	value int
	ref   *Reference
	match bool
	err   error
}

type partRule struct {
	min      int
	max      int
	class    Class
	empty    partResult
	describe func(token string, min, max int) string
}

var (
	numberPattern    = regexp.MustCompile(`^\d+$`)
	relativePattern  = regexp.MustCompile(`^@(?:\.([A-Za-z][A-Za-z0-9]*))?$`)
	bareWordPattern  = regexp.MustCompile(`^(?:[dD]|[kK][hHlL]\d*)$`)
	pathPattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)*$`)
	minSuffixPattern = regexp.MustCompile(`[ \t]+min[ \t]+([0-9]+)[ \t]*$`)
)

const whitespace = " \t"

func Parse(expr string) (Expression, error) {
	if length := utf8.RuneCountInString(expr); length > MaxLength {
		return Expression{}, newError(ClassTooLong,
			fmt.Sprintf("the expression has %d characters (limit %d)", length, MaxLength))
	}
	if strings.Trim(expr, whitespace) == "" {
		return Expression{}, ErrEmpty
	}

	minimum, sum, err := splitMinimum(expr)
	if err != nil {
		return Expression{}, err
	}

	pieces := splitOperators(sum)
	terms := make([]Term, 0, len(pieces)/2+1)
	index, sign := 0, 1
	if pieces[0] == "" && len(pieces) > 1 {
		sign = operatorSign(pieces[1])
		index = 2
	}
	for index < len(pieces) {
		term, err := parseTerm(pieces[index])
		if err != nil {
			return Expression{}, err
		}
		term.Sign = sign
		terms = append(terms, term)
		index++
		if index < len(pieces) {
			sign = operatorSign(pieces[index])
			index++
		}
	}
	if len(terms) > MaxTerms {
		return Expression{}, newError(ClassTooManyTerms,
			fmt.Sprintf("the expression has %d terms (limit %d)", len(terms), MaxTerms))
	}
	return Expression{terms: terms, minimum: minimum}, nil
}

func (e Expression) Terms() []Term {
	return append([]Term(nil), e.terms...)
}

func (e Expression) Minimum() (int, bool) {
	if e.minimum == nil {
		return 0, false
	}
	return *e.minimum, true
}

func (e Expression) String() string {
	var b strings.Builder
	for i, term := range e.terms {
		switch {
		case i == 0 && term.Sign < 0:
			b.WriteString("-")
		case i > 0 && term.Sign < 0:
			b.WriteString(" - ")
		case i > 0:
			b.WriteString(" + ")
		}
		b.WriteString(term.label(""))
	}
	if e.minimum != nil {
		b.WriteString(" min ")
		b.WriteString(strconv.Itoa(*e.minimum))
	}
	return b.String()
}

func (t Term) label(rowKey string) string {
	switch t.Kind {
	case KindDice:
		count := strconv.Itoa(t.Count)
		if t.CountRef != nil {
			count = "{" + t.CountRef.fieldPath(rowKey, true) + "}"
		}
		faces := strconv.Itoa(t.Faces)
		if t.FacesRef != nil {
			faces = "{" + t.FacesRef.fieldPath(rowKey, true) + "}"
		}
		return count + "d" + faces
	case KindNumber:
		return strconv.Itoa(t.Value)
	default:
		return t.fieldPath(rowKey, true)
	}
}

func (t Term) fieldPath(rowKey string, symbolic bool) string {
	return Reference{Path: t.Path, Relative: t.Relative, Column: t.Column}.fieldPath(rowKey, symbolic)
}

func (r Reference) fieldPath(rowKey string, symbolic bool) string {
	if !r.Relative {
		return r.Path
	}
	if symbolic {
		if r.Column == "" {
			return "@"
		}
		return "@." + r.Column
	}
	if r.Column == "" {
		return rowKey
	}
	return rowKey + "." + r.Column
}

func splitMinimum(expr string) (*int, string, error) {
	match := minSuffixPattern.FindStringSubmatchIndex(expr)
	if match == nil {
		return nil, expr, nil
	}
	digits := expr[match[2]:match[3]]
	value, ok := boundedInt(digits, MaxWholeNumber)
	if !ok {
		return nil, "", newError(ClassNumberRange,
			fmt.Sprintf("%q: minimums go up to %d", expr[match[0]:match[1]], MaxWholeNumber))
	}
	sum := strings.Trim(expr[:match[0]], whitespace)
	if sum == "" {
		return nil, "", ErrMissingTerm
	}
	return &value, sum, nil
}

func splitOperators(expr string) []string {
	pieces := make([]string, 0, 2*MaxTerms)
	start := 0
	for i := 0; i < len(expr); i++ {
		if expr[i] != '+' && expr[i] != '-' {
			continue
		}
		pieces = append(pieces, strings.Trim(expr[start:i], whitespace), expr[i:i+1])
		start = i + 1
	}
	return append(pieces, strings.Trim(expr[start:], whitespace))
}

func operatorSign(op string) int {
	if op == "-" {
		return -1
	}
	return 1
}

func parseTerm(token string) (Term, error) {
	if token == "" {
		return Term{}, ErrMissingTerm
	}
	if term, ok, err := parseDiceToken(token); err != nil {
		return Term{}, err
	} else if ok {
		return term, nil
	}
	if numberPattern.MatchString(token) {
		value, ok := boundedInt(token, MaxWholeNumber)
		if !ok {
			return Term{}, newError(ClassNumberRange,
				fmt.Sprintf("%q: whole numbers go up to %d", token, MaxWholeNumber))
		}
		return Term{Kind: KindNumber, Value: value}, nil
	}
	if match := relativePattern.FindStringSubmatch(token); match != nil {
		return Term{Kind: KindField, Relative: true, Column: match[1]}, nil
	}
	if bareWordPattern.MatchString(token) {
		return Term{}, newError(ClassUnreadableTerm,
			fmt.Sprintf("cannot read %q: a die needs its faces (NdM) and keep modifiers are not supported", token))
	}
	if pathPattern.MatchString(token) {
		return Term{Kind: KindField, Path: token}, nil
	}
	return Term{}, newError(ClassUnreadableTerm,
		fmt.Sprintf("cannot read %q: only NdM, whole numbers, Field paths and @, joined by + or -", token))
}

func parseDiceToken(token string) (Term, bool, error) {
	countPart, facesPart, ok := splitDiceParts(token)
	if !ok {
		return Term{}, false, nil
	}
	count := parsePart(token, countPart, countPartRule)
	if !count.match {
		return Term{}, false, nil
	}
	if count.err != nil {
		return Term{}, false, count.err
	}
	faces := parsePart(token, facesPart, facesPartRule)
	if !faces.match {
		return Term{}, false, nil
	}
	if faces.err != nil {
		return Term{}, false, faces.err
	}
	return Term{
		Kind:     KindDice,
		Count:    count.value,
		Faces:    faces.value,
		CountRef: count.ref,
		FacesRef: faces.ref,
	}, true, nil
}

func splitDiceParts(token string) (string, string, bool) {
	depth := 0
	for i := 0; i < len(token); i++ {
		switch token[i] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return "", "", false
			}
			depth--
		case 'd', 'D':
			if depth == 0 {
				return token[:i], token[i+1:], true
			}
		}
	}
	return "", "", false
}

var (
	countPartRule = partRule{
		min:   MinDice,
		max:   MaxDice,
		class: ClassDiceCount,
		empty: partResult{value: 1, match: true},
		describe: func(token string, min, max int) string {
			return fmt.Sprintf("%q: roll between %d and %d dice at a time", token, min, max)
		},
	}
	facesPartRule = partRule{
		min:   MinFaces,
		max:   MaxFaces,
		class: ClassDiceFaces,
		describe: func(token string, min, max int) string {
			return fmt.Sprintf("%q: a die has between %d and %d faces", token, min, max)
		},
	}
)

func parsePart(token, part string, rule partRule) partResult {
	if part == "" {
		return rule.empty
	}
	if ref, ok := parseReference(part); ok {
		return partResult{ref: ref, match: true}
	}
	if !numberPattern.MatchString(part) {
		return partResult{}
	}
	value, ok := boundedInt(part, rule.max)
	if !ok || value < rule.min {
		return partResult{match: true, err: newError(rule.class, rule.describe(token, rule.min, rule.max))}
	}
	return partResult{value: value, match: true}
}

func parseReference(token string) (*Reference, bool) {
	if len(token) < 3 || token[0] != '{' || token[len(token)-1] != '}' {
		return nil, false
	}
	inner := token[1 : len(token)-1]
	if strings.ContainsAny(inner, "{}") {
		return nil, false
	}
	if match := relativePattern.FindStringSubmatch(inner); match != nil {
		return &Reference{Relative: true, Column: match[1]}, true
	}
	if pathPattern.MatchString(inner) {
		return &Reference{Path: inner}, true
	}
	return nil, false
}

func boundedInt(digits string, limit int) (int, bool) {
	value := 0
	for i := 0; i < len(digits); i++ {
		value = value*10 + int(digits[i]-'0')
		if value > limit {
			return 0, false
		}
	}
	return value, true
}
