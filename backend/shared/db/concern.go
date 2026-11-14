package db

import (
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

// TransactionOptions returns the options for every multi-document transaction.
// Writes use w:majority so a committed transaction survives a failover, and
// the primary serves the reads so the transaction sees its own writes.
func TransactionOptions() *options.TransactionOptionsBuilder {
	return options.Transaction().
		SetWriteConcern(writeconcern.Majority()).
		SetReadPreference(readpref.Primary())
}

// PrimaryReadPreference returns the read preference for reads that decide
// authorization or a compensation. A secondary may lag behind the last
// committed write, so those reads must go to the primary.
func PrimaryReadPreference() *readpref.ReadPref {
	return readpref.Primary()
}
