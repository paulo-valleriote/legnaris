package template

import "fmt"

var (
	ErrCompileFormula = func(formula string, nodeID string, err error) error {
		return fmt.Errorf("failed to compile formula '%s' on node '%s': %v", formula, nodeID, err)
	}
	ErrFormulaEval = func(err error) error {
		return fmt.Errorf("failed to evaluate formula: %v", err)
	}
)
