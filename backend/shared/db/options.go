package db

import (
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

// TransactionOptions returns the options WithTransaction applies to every
// multi-document transaction. Writes use w:majority so a committed transaction
// survives a failover, and the primary serves the reads so the transaction sees
// its own writes. Each call returns a fresh builder.
func TransactionOptions() *options.TransactionOptionsBuilder {
	return options.Transaction().
		SetWriteConcern(writeconcern.Majority()).
		SetReadPreference(readpref.Primary())
}
