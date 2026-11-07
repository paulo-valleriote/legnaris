package dice

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Limits of the grammar. They are mirrored in the shared test vectors, which
// pin them for both implementations.
const (
	MaxLength      = 64
	MaxTerms       = 8
	MinDice        = 1
	MaxDice        = 20
	MinFaces       = 1
	MaxFaces       = 1000
	MaxWholeNumber = 9999
)

// Kind tells what a Term is.
type Kind int

const (
	// KindDice is an NdM term, rolled when the expression is evaluated.
	KindDice Kind = iota + 1
	// KindNumber is a whole number literal.
	KindNumber
	// KindField is a reference to a numeric Field, by path or relative to the Row.
	KindField
)

// Term is one signed operand of a Dice expression.
type Term struct {
	Kind Kind
	// Sign is +1 or -1.
	Sign int
	// Count and Faces describe a KindDice term (Count is 1 for "d20").
	Count int
	Faces int
	// Value is the whole number of a KindNumber term.
	Value int
	// Path is the dotted Field path of a KindField term that is not Relative.
	Path string
	// Relative marks "@" / "@.<Column>": the Field of the Row being rolled.
	Relative bool
	Column   string
}

// Expression is a parsed Dice expression. It is immutable, so it is safe to
// share between goroutines; the zero value has no terms.
type Expression struct {
	terms []Term
}

var (
	diceTermPattern = regexp.MustCompile(`^(\d*)[dD](\d+)$`)
	numberPattern   = regexp.MustCompile(`^\d+$`)
	relativePattern = regexp.MustCompile(`^@(?:\.([A-Za-z][A-Za-z0-9]*))?$`)
	// bareWordPattern matches a whole single-segment token that is a die letter
	// or a keep modifier without its dice, which a Field path would otherwise swallow.
	bareWordPattern = regexp.MustCompile(`^(?:[dD]|[kK][hHlL]\d*)$`)
	pathPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)*$`)
)

const whitespace = " \t"

// Parse reads a Dice expression: dice (`NdM`, `dM` means `1dM`), whole numbers
// and numeric Field paths (or `@` / `@.<column>` for the Row being rolled)
// joined by `+` and `-`, with an optional leading sign. Only space and tab are
// whitespace, and only between terms and operators; `D` is read like `d`.
// Parse checks syntax and limits only; whether the referenced Fields exist and
// are numeric is decided by Expression.FitsRow.
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

// Terms returns a copy of the expression's terms in order.
func (e Expression) Terms() []Term {
	return append([]Term(nil), e.terms...)
}

// String returns the canonical form: single spaces around operators, a lower
// case `d`, explicit dice counts and no leading zeros or leading `+`.
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

// splitOperators mirrors a split on `+`/`-` that keeps the operators: operands
// sit at even indexes (trimmed, possibly empty) and operators at odd ones.
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

// boundedInt parses ASCII digits and reports false when the value exceeds limit,
// without ever overflowing on absurdly long inputs.
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
