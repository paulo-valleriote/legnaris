package engine

import (
	"fmt"
	"math"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type FormulaEngine struct {
	compileOptions []expr.Option
}

func NewFormulaEngine() *FormulaEngine {
	return &FormulaEngine{
		compileOptions: []expr.Option{
			expr.Function("floor", floorFunc),
			expr.Function("max", maxFunc),
			expr.Function("min", minFunc),
			expr.Function("lookup", lookupFunc),
		},
	}
}

func (fe *FormulaEngine) Compile(formula string, mockEnv map[string]any) (*vm.Program, error) {
	if formula == "" {
		return nil, nil
	}

	options := append([]expr.Option{}, fe.compileOptions...)
	options = append(options, expr.Env(mockEnv))

	program, err := expr.Compile(formula, options...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCompile, err)
	}
	return program, nil
}

func (fe *FormulaEngine) Evaluate(program *vm.Program, env map[string]any) (any, error) {
	if program == nil {
		return nil, ErrNilProgram
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEvaluate, err)
	}

	return result, nil
}

func coerceFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("%w: expected a numeric type, got %T", ErrInvalidFloat, v)
	}
}

func floorFunc(params ...any) (any, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("%w: floor expects exactly 1 argument", ErrInvalidFloat)
	}

	v, err := coerceFloat64(params[0])
	if err != nil {
		return nil, err
	}
	return math.Floor(v), nil
}

func maxFunc(params ...any) (any, error) {
	return numericPair("max", math.Max, params)
}

func minFunc(params ...any) (any, error) {
	return numericPair("min", math.Min, params)
}

func numericPair(name string, combine func(float64, float64) float64, params []any) (any, error) {
	if len(params) != 2 {
		return nil, fmt.Errorf("%w: %s expects exactly 2 arguments", ErrInvalidFloat, name)
	}

	a, err := coerceFloat64(params[0])
	if err != nil {
		return nil, fmt.Errorf("%w: first argument must be numeric", ErrInvalidFloat)
	}

	b, err := coerceFloat64(params[1])
	if err != nil {
		return nil, fmt.Errorf("%w: second argument must be numeric", ErrInvalidFloat)
	}

	return combine(a, b), nil
}

func lookupFunc(params ...any) (any, error) {
	if len(params) < 3 || len(params)%2 == 0 {
		return nil, fmt.Errorf("%w: lookup expects an input and at least one key/value pair", ErrInvalidLookupArity)
	}

	x, err := coerceFloat64(params[0])
	if err != nil {
		return nil, fmt.Errorf("%w: input must be numeric", ErrInvalidFloat)
	}

	result := 0.0
	previousKey := 0.0
	for i := range (len(params) - 1) / 2 {
		key, err := coerceFloat64(params[1+2*i])
		if err != nil {
			return nil, fmt.Errorf("%w: key %d must be numeric", ErrInvalidFloat, i+1)
		}

		value, err := coerceFloat64(params[2+2*i])
		if err != nil {
			return nil, fmt.Errorf("%w: value %d must be numeric", ErrInvalidFloat, i+1)
		}

		if i > 0 && key <= previousKey {
			return nil, fmt.Errorf("%w: key %d is not greater than key %d", ErrInvalidLookupKeyOrder, i+1, i)
		}

		if i == 0 || key <= x {
			result = value
		}
		previousKey = key
	}

	return result, nil
}
