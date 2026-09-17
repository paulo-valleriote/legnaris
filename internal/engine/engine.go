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
			expr.Function("floor", floor),
			expr.Function("max", max),
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

func floor(params ...any) (any, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("%w: floor expects exactly 1 argument", ErrInvalidFLoat)
	}

	var val float64
	switch v := params[0].(type) {
	case float64:
		val = v
	case int:
		val = float64(v)
	default:
		return nil, fmt.Errorf("%w: expected float64 or int", ErrInvalidFLoat)
	}
	return math.Floor(val), nil
}

func max(params ...any) (any, error) {
	if len(params) != 2 {
		return nil, fmt.Errorf("%w: max expects exactly 2 arguments", ErrInvalidFLoat)
	}

	a, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("%w: first argument must be float", ErrInvalidFLoat)
	}

	b, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("%w: second argument must be float", ErrInvalidFLoat)
	}

	return math.Max(a, b), nil
}
