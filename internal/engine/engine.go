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

// coerceFloat64 normalizes any Go numeric type to float64 so that floor/max
// handle int, int32, int64, float32, etc. consistently regardless of whether
// values arrive via JSON (float64) or Mongo BSON (int32/int64).
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
	if len(params) != 2 {
		return nil, fmt.Errorf("%w: max expects exactly 2 arguments", ErrInvalidFloat)
	}

	a, err := coerceFloat64(params[0])
	if err != nil {
		return nil, fmt.Errorf("%w: first argument must be numeric", ErrInvalidFloat)
	}

	b, err := coerceFloat64(params[1])
	if err != nil {
		return nil, fmt.Errorf("%w: second argument must be numeric", ErrInvalidFloat)
	}

	return math.Max(a, b), nil
}
