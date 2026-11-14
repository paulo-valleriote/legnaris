package db

import (
	"testing"

	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientOptions_RetryableWrites(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "Happy path: replica set URI with retryWrites enables retryable writes", uri: "mongodb://127.0.0.1:27017/?replicaSet=rs0&retryWrites=true"},
		{name: "Edge: URI without retryWrites still enables them", uri: "mongodb://127.0.0.1:27017/?replicaSet=rs0"},
		{name: "Edge: URI that disables retryWrites is overridden", uri: "mongodb://127.0.0.1:27017/?replicaSet=rs0&retryWrites=false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := newClientOptions(env.Variables{DatabaseURI: tc.uri})

			require.NotNil(t, opts.RetryWrites)
			assert.True(t, *opts.RetryWrites)
		})
	}
}

func TestNewClientOptions_ReplicaSetFromURI(t *testing.T) {
	t.Run("Happy path: replicaSet from the URI reaches the client options", func(t *testing.T) {
		opts := newClientOptions(env.Variables{DatabaseURI: "mongodb://127.0.0.1:27017/?replicaSet=rs0"})

		require.NotNil(t, opts.ReplicaSet)
		assert.Equal(t, "rs0", *opts.ReplicaSet)
	})

	t.Run("Edge: standalone URI leaves the replica set unset", func(t *testing.T) {
		opts := newClientOptions(env.Variables{DatabaseURI: "mongodb://127.0.0.1:27017/"})

		assert.Nil(t, opts.ReplicaSet)
	})
}
