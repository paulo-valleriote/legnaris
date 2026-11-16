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

type Term struct {
	Kind     Kind
	Sign     int
	Count    int
	Faces    int
	Value    int
	Path     string
	Relative bool
	Column   string
}

type Expression struct {
	terms []Term
}

var (
	diceTermPattern = regexp.MustCompile(`^(\d*)[dD](\d+)$`)
	numberPattern   = regexp.MustCompile(`^\d+$`)
	relativePattern = regexp.MustCompile(`^@(?:\.([A-Za-z][A-Za-z0-9]*))?$`)
	bareWordPattern = regexp.MustCompile(`^(?:[dD]|[kK][hHlL]\d*)$`)
	pathPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)*$`)
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

	pieces := splitOperators(expr)
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
	return Expression{terms: terms}, nil
}

func (e Expression) Terms() []Term {
	return append([]Term(nil), e.terms...)
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
	return b.String()
}

func (t Term) label(rowKey string) string {
	switch t.Kind {
	case KindDice:
		return strconv.Itoa(t.Count) + "d" + strconv.Itoa(t.Faces)
	case KindNumber:
		return strconv.Itoa(t.Value)
	default:
		return t.fieldPath(rowKey, true)
	}
}

func (t Term) fieldPath(rowKey string, symbolic bool) string {
	if !t.Relative {
		return t.Path
	}
	if symbolic {
		if t.Column == "" {
			return "@"
		}
		return "@." + t.Column
	}
	if t.Column == "" {
		return rowKey
	}
	return rowKey + "." + t.Column
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
	if match := diceTermPattern.FindStringSubmatch(token); match != nil {
		return parseDice(token, match[1], match[2])
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

func parseDice(token, countDigits, facesDigits string) (Term, error) {
	count := 1
	if countDigits != "" {
		parsed, ok := boundedInt(countDigits, MaxDice)
		if !ok || parsed < MinDice {
			return Term{}, newError(ClassDiceCount,
				fmt.Sprintf("%q: roll between %d and %d dice at a time", token, MinDice, MaxDice))
		}
		count = parsed
	}
	faces, ok := boundedInt(facesDigits, MaxFaces)
	if !ok || faces < MinFaces {
		return Term{}, newError(ClassDiceFaces,
			fmt.Sprintf("%q: a die has between %d and %d faces", token, MinFaces, MaxFaces))
	}
	return Term{Kind: KindDice, Count: count, Faces: faces}, nil
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
