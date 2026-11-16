package db

import (
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

func TransactionOptions() *options.TransactionOptionsBuilder {
	return options.Transaction().
		SetWriteConcern(writeconcern.Majority()).
		SetReadPreference(readpref.Primary())
}
