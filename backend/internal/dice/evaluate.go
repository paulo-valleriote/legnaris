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
	Total   float64
	Parts   []Part
	Minimum *int
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
	if e.minimum != nil && result.Total < float64(*e.minimum) {
		result.Total = float64(*e.minimum)
		minimum := *e.minimum
		result.Minimum = &minimum
	}
	return result, nil
}

func evaluateTerm(term Term, rowKey string, values map[string]float64, roll Roller) (Part, error) {
	part := Part{Sign: term.Sign}
	switch term.Kind {
	case KindDice:
		count, err := diceCount(term, rowKey, values)
		if err != nil {
			return Part{}, err
		}
		faces, err := diceFaces(term, rowKey, values)
		if err != nil {
			return Part{}, err
		}
		if roll == nil {
			return Part{}, ErrNoRoller
		}
		part.Label = fmt.Sprintf("%dd%d", count, faces)
		part.Rolls = make([]int, count)
		for i := range part.Rolls {
			part.Rolls[i] = roll(faces)
			part.Value += float64(part.Rolls[i])
		}
	case KindNumber:
		part.Label = term.label(rowKey)
		part.Value = float64(term.Value)
	default:
		path := term.fieldPath(rowKey, false)
		part.Label = path
		value, err := readField(values, path)
		if err != nil {
			return Part{}, err
		}
		part.Value = value
	}
	return part, nil
}

func diceCount(term Term, rowKey string, values map[string]float64) (int, error) {
	if term.CountRef == nil {
		return term.Count, nil
	}
	path := term.CountRef.fieldPath(rowKey, false)
	value, err := readField(values, path)
	if err != nil {
		return 0, err
	}
	if value < MinDice || value > MaxDice {
		return 0, newError(ClassDiceCount,
			fmt.Sprintf("%q gives %v dice: roll between %d and %d dice at a time", path, value, MinDice, MaxDice))
	}
	return int(value), nil
}

func diceFaces(term Term, rowKey string, values map[string]float64) (int, error) {
	if term.FacesRef == nil {
		return term.Faces, nil
	}
	path := term.FacesRef.fieldPath(rowKey, false)
	value, err := readField(values, path)
	if err != nil {
		return 0, err
	}
	if value < MinFaces || value > MaxFaces {
		return 0, newError(ClassDiceFaces,
			fmt.Sprintf("%q gives %v faces: a die has between %d and %d faces", path, value, MinFaces, MaxFaces))
	}
	return int(value), nil
}

func readField(values map[string]float64, path string) (float64, error) {
	raw, ok := values[path]
	if !ok || math.IsNaN(raw) || math.IsInf(raw, 0) {
		return 0, newError(ClassNoValue, fmt.Sprintf("%q has no number to read", path))
	}
	return math.Floor(raw), nil
}
