package dice

import (
	"fmt"
	"math"
)

type Roller func(faces int) int

type Part struct {
	Sign  int
	Label string
	Rolls []int
	Value float64
}

type Result struct {
	Total float64
	Parts []Part
}

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
