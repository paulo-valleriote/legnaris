package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransactor_EffectiveTimeout(t *testing.T) {
	tests := []struct {
		name string
		tx   *Transactor
		want time.Duration
	}{
		{name: "Happy path: a new transactor uses the default cap", tx: NewTransactor(nil, nil), want: DefaultTransactionTimeout},
		{name: "Happy path: a positive timeout replaces the default", tx: NewTransactor(nil, nil).WithTimeout(time.Second), want: time.Second},
		{name: "Edge: a zero timeout keeps the default", tx: NewTransactor(nil, nil).WithTimeout(0), want: DefaultTransactionTimeout},
		{name: "Edge: a negative timeout keeps the default", tx: NewTransactor(nil, nil).WithTimeout(-time.Second), want: DefaultTransactionTimeout},
		{name: "Edge: a zero-value transactor falls back to the default", tx: &Transactor{}, want: DefaultTransactionTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.tx.effectiveTimeout())
		})
	}
}
