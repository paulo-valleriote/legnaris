package dice

import "errors"

// Class names one reason a Dice expression is refused. The string values are
// part of the shared test vectors (testdata/dice/vectors.json) and must match
// the TypeScript implementation.
type Class string

const (
	// ClassEmpty reports that expression is empty or only whitespace.
	ClassEmpty Class = "empty"
	// ClassTooLong reports that expression exceeds the character limit.
	ClassTooLong Class = "too_long"
	// ClassTooManyTerms reports that expression exceeds the term limit.
	ClassTooManyTerms Class = "too_many_terms"
	// ClassDiceCount reports that dice term rolls fewer than one or more than the allowed number of dice.
	ClassDiceCount Class = "dice_count"
	// ClassDiceFaces reports that dice term has fewer than one or more than the allowed number of faces.
	ClassDiceFaces Class = "dice_faces"
	// ClassNumberRange reports that whole number exceeds the allowed maximum.
	ClassNumberRange Class = "number_range"
	// ClassMissingTerm reports that operator has no term on one side of it.
	ClassMissingTerm Class = "missing_term"
	// ClassUnreadableTerm reports that term matches none of the accepted forms.
	ClassUnreadableTerm Class = "unreadable_term"
	// ClassUnknownField reports that Field path names no Field in the Template.
	ClassUnknownField Class = "unknown_field"
	// ClassNonNumericField reports that Field path names a Field that is not a number.
	ClassNonNumericField Class = "non_numeric_field"
	// ClassNoRows reports that Section has no Rows to check the expression against.
	ClassNoRows Class = "no_rows"
	// ClassNoValue reports that Field has no finite number to read, or the total overflows.
	ClassNoValue Class = "no_value"
	// ClassNoRoller reports that expression has dice but no Roller was supplied.
	ClassNoRoller Class = "no_roller"
)

// Error is the typed error of this package. The package deliberately does not
// import apperr: the boundary that exposes a Dice expression translates the
// Class into an apperr code.
type Error struct {
	Class   Class
	Message string
}

// Error returns the human-readable reason.
func (e *Error) Error() string {
	return e.Message
}

// Is reports whether target is an *Error of the same Class, so callers can
// match with errors.Is against the Err* sentinels.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && other.Class == e.Class
}

var (
	// ErrEmpty is returned when the expression is empty.
	ErrEmpty = &Error{Class: ClassEmpty, Message: "the expression is empty"}
	// ErrTooLong is returned when the expression is too long.
	ErrTooLong = &Error{Class: ClassTooLong, Message: "the expression is too long"}
	// ErrTooManyTerms is returned when the expression has too many terms.
	ErrTooManyTerms = &Error{Class: ClassTooManyTerms, Message: "the expression has too many terms"}
	// ErrDiceCount is returned when the number of dice is out of range.
	ErrDiceCount = &Error{Class: ClassDiceCount, Message: "the number of dice is out of range"}
	// ErrDiceFaces is returned when the number of faces is out of range.
	ErrDiceFaces = &Error{Class: ClassDiceFaces, Message: "the number of faces is out of range"}
	// ErrNumberRange is returned when the whole number is out of range.
	ErrNumberRange = &Error{Class: ClassNumberRange, Message: "the whole number is out of range"}
	// ErrMissingTerm is returned when a term is missing next to a + or -.
	ErrMissingTerm = &Error{Class: ClassMissingTerm, Message: "a term is missing next to a + or -"}
	// ErrUnreadableTerm is returned when the term cannot be read.
	ErrUnreadableTerm = &Error{Class: ClassUnreadableTerm, Message: "the term cannot be read"}
	// ErrUnknownField is returned when the Field does not exist.
	ErrUnknownField = &Error{Class: ClassUnknownField, Message: "the Field does not exist"}
	// ErrNonNumericField is returned when the Field is not a number.
	ErrNonNumericField = &Error{Class: ClassNonNumericField, Message: "the Field is not a number"}
	// ErrNoRows is returned when the Section has no Rows.
	ErrNoRows = &Error{Class: ClassNoRows, Message: "the Section has no Rows"}
	// ErrNoValue is returned when the Field has no number to read.
	ErrNoValue = &Error{Class: ClassNoValue, Message: "the Field has no number to read"}
	// ErrNoRoller is returned when the expression has dice but no roller was given.
	ErrNoRoller = &Error{Class: ClassNoRoller, Message: "the expression has dice but no roller was given"}
)

// ClassOf returns the Class of the first *Error in err's chain.
func ClassOf(err error) (Class, bool) {
	if typed, ok := errors.AsType[*Error](err); ok {
		return typed.Class, true
	}
	return "", false
}

func newError(class Class, message string) *Error {
	return &Error{Class: class, Message: message}
}
