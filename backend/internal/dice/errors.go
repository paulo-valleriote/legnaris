package dice

import "errors"

// Class names one reason a Dice expression is refused. The string values are
// part of the shared test vectors (testdata/dice/vectors.json) and must match
// the TypeScript implementation.
type Class string

const (
	ClassEmpty           Class = "empty"
	ClassTooLong         Class = "too_long"
	ClassTooManyTerms    Class = "too_many_terms"
	ClassDiceCount       Class = "dice_count"
	ClassDiceFaces       Class = "dice_faces"
	ClassNumberRange     Class = "number_range"
	ClassMissingTerm     Class = "missing_term"
	ClassUnreadableTerm  Class = "unreadable_term"
	ClassUnknownField    Class = "unknown_field"
	ClassNonNumericField Class = "non_numeric_field"
	ClassNoRows          Class = "no_rows"
	ClassNoValue         Class = "no_value"
	ClassNoRoller        Class = "no_roller"
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
	ErrEmpty           = &Error{Class: ClassEmpty, Message: "the expression is empty"}
	ErrTooLong         = &Error{Class: ClassTooLong, Message: "the expression is too long"}
	ErrTooManyTerms    = &Error{Class: ClassTooManyTerms, Message: "the expression has too many terms"}
	ErrDiceCount       = &Error{Class: ClassDiceCount, Message: "the number of dice is out of range"}
	ErrDiceFaces       = &Error{Class: ClassDiceFaces, Message: "the number of faces is out of range"}
	ErrNumberRange     = &Error{Class: ClassNumberRange, Message: "the whole number is out of range"}
	ErrMissingTerm     = &Error{Class: ClassMissingTerm, Message: "a term is missing next to a + or -"}
	ErrUnreadableTerm  = &Error{Class: ClassUnreadableTerm, Message: "the term cannot be read"}
	ErrUnknownField    = &Error{Class: ClassUnknownField, Message: "the Field does not exist"}
	ErrNonNumericField = &Error{Class: ClassNonNumericField, Message: "the Field is not a number"}
	ErrNoRows          = &Error{Class: ClassNoRows, Message: "the Section has no Rows"}
	ErrNoValue         = &Error{Class: ClassNoValue, Message: "the Field has no number to read"}
	ErrNoRoller        = &Error{Class: ClassNoRoller, Message: "the expression has dice but no roller was given"}
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
