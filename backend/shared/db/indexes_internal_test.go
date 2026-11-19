package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestIsIndexNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Happy path: the IndexNotFound command error is tolerated",
			err:  mongo.CommandError{Code: 27, Message: "index not found"},
			want: true,
		},
		{
			name: "Edge: a wrapped IndexNotFound command error is tolerated",
			err:  fmt.Errorf("drop index: %w", mongo.CommandError{Code: 27, Message: "index not found"}),
			want: true,
		},
		{
			name: "Edge: NamespaceNotFound is not an index miss",
			err:  mongo.CommandError{Code: 26, Message: "namespace not found"},
			want: false,
		},
		{
			name: "Edge: an unrelated command error is not an index miss",
			err:  mongo.CommandError{Code: 13, Message: "unauthorized"},
			want: false,
		},
		{
			name: "Edge: a plain error is not an index miss",
			err:  errors.New("network down"),
			want: false,
		},
		{
			name: "Edge: a nil error is not an index miss",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isIndexNotFound(tc.err))
		})
	}
}
