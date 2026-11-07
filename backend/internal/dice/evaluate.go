package dice

import (
	"fmt"
	"math"
)

// Roller returns one die result in [1, faces]. The caller owns the random
// source, which keeps Evaluate deterministic under test; Evaluate trusts the
// range.
type Roller func(faces int) int

// Part is one term of a Roll: what it was, its sign, and the value it
// contributed before the sign is applied.
type Part struct {
	Sign  int
	Label string
	// Rolls holds every die of a dice term, in the order they were thrown.
	Rolls []int
	Value float64
}

// Result is the outcome of an Evaluate.
type Result struct {
	Total float64
	Parts []Part
}

// Evaluate computes the expression for the Row keyed rowKey. Dice are thrown in
// expression order through roll; a Field is read from values by its resolved
// path and floored, like Template formulas do. It returns ErrNoValue for a
// Field that is absent, NaN or infinite, ErrOverflow when the total grows to
// infinity, and ErrNoRoller when the expression has dice and roll is nil. It does not check the schema: use FitsRow for that.
func (e Expression) Evaluate(rowKey string, values map[string]float64, roll Roller) (Result, error) {
	result := Result{Parts: make([]Part, 0, len(e.terms))}
	for _, term := range e.terms {
		part, err := evaluateTerm(term, rowKey, values, roll)
		if err != nil {
			return Result{}, err
		}
		result.Total += float64(part.Sign) * part.Value
		if math.IsInf(result.Total, 0) {
			return Result{}, ErrOverflow
		}
		result.Parts = append(result.Parts, part)
	}
	return result, nil
}

func evaluateTerm(term Term, rowKey string, values map[string]float64, roll Roller) (Part, error) {
	part := Part{Sign: term.Sign, Label: term.label(rowKey)}
	switch term.Kind {
	case KindDice:
		if roll == nil {
			return Part{}, ErrNoRoller
		}
		part.Rolls = make([]int, term.Count)
		for i := range part.Rolls {
			part.Rolls[i] = roll(term.Faces)
			part.Value += float64(part.Rolls[i])
		}
	case KindNumber:
		part.Value = float64(term.Value)
	default:
		path := term.fieldPath(rowKey, false)
		part.Label = path
		raw, ok := values[path]
		if !ok || math.IsNaN(raw) || math.IsInf(raw, 0) {
			return Part{}, newError(ClassNoValue, fmt.Sprintf("%q has no number to read", path))
		}
		part.Value = math.Floor(raw)
	}
	return part, nil
}
