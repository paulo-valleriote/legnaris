// Package dice parses and validates Dice expressions and decides whether one
// fits a Row.
//
// The grammar is its own, deliberately not expr-lang: a player can type a Dice
// expression, so the formula engine must never evaluate it (ADR 0005). The web
// client holds the other implementation; testdata/dice/vectors.json keeps the
// two in step.
//
// The package returns its own *Error values and does not import apperr; the
// boundary that stores or serves an expression translates the Class.
package dice
