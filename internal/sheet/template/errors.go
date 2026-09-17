package template

import (
	"errors"
	"fmt"
)

var (
	ErrCompileFormula = func(formula string, nodeID string, err error) error {
		return fmt.Errorf("failed to compile formula '%s' on node '%s': %w", formula, nodeID, err)
	}
	ErrInfiniteCycle = errors.New("An infinite cycle was detected on template formulas")
)
