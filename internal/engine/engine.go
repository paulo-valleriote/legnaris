package engine

import (
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

	return expr.Compile(formula, options...)
}

func (fe *FormulaEngine) Evaluate(program *vm.Program, env map[string]any) (any, error) {
	if program == nil {
		return nil, ErrNilProgram
	}
	return expr.Run(program, env)
}

func floor(params ...any) (any, error) {
	var val float64
	switch v := params[0].(type) {
	case float64:
		val = v
	case int:
		val = float64(v)
	default:
		return nil, ErrInvalidFLoat
	}
	return math.Floor(val), nil
}

func max(params ...any) (any, error) {
	a, ok := params[0].(float64)
	if !ok {
		return nil, ErrInvalidFLoat
	}

	b, ok := params[1].(float64)
	if !ok {
		return nil, ErrInvalidFLoat
	}

	return math.Max(a, b), nil
}
