package db_test

import (
	"testing"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func TestTransactionOptions(t *testing.T) {
	tests := []struct {
		name string
		make func() *options.TransactionOptionsBuilder
	}{
		{name: "Happy path: transactions write with majority and read from the primary", make: db.TransactionOptions},
		{name: "Edge: each call returns settings with the same values", make: func() *options.TransactionOptionsBuilder { return db.TransactionOptions() }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applied := &options.TransactionOptions{}
			for _, set := range tc.make().List() {
				require.NoError(t, set(applied))
			}

			require.NotNil(t, applied.WriteConcern)
			assert.Equal(t, "majority", applied.WriteConcern.W)
			require.NotNil(t, applied.ReadPreference)
			assert.Equal(t, readpref.PrimaryMode, applied.ReadPreference.Mode())
		})
	}
}
