package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	fieldEventType = "event_type"
	fieldBody      = "body"
	fieldData      = "data"
	fieldKind      = "kind"
	operandIn      = "$in"
)

func RewritePendingDataKind(ctx context.Context, database *mongo.Database, eventTypes []string, from, to string) (int64, error) {
	if len(eventTypes) == 0 || from == "" || to == "" || from == to {
		return 0, nil
	}

	collection := database.Collection(eventOutboxCollection)
	cursor, err := collection.Find(ctx, bson.D{
		{Key: fieldStatus, Value: StatusPending},
		{Key: fieldEventType, Value: bson.D{{Key: operandIn, Value: eventTypes}}},
	})
	if err != nil {
		return 0, fmt.Errorf("scan pending event outbox entries: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var rewritten int64
	for cursor.Next(ctx) {
		var entry Entry
		if err := cursor.Decode(&entry); err != nil {
			return rewritten, fmt.Errorf("decode pending event outbox entry: %w", err)
		}
		body, changed, err := rewriteBodyKind(entry.Body, from, to)
		if err != nil {
			return rewritten, err
		}
		if !changed {
			continue
		}
		result, err := collection.UpdateOne(ctx,
			bson.D{
				{Key: fieldID, Value: entry.ID},
				{Key: fieldStatus, Value: StatusPending},
				{Key: fieldBody, Value: entry.Body},
			},
			bson.D{{Key: operandSet, Value: bson.D{{Key: fieldBody, Value: body}}}},
		)
		if err != nil {
			return rewritten, fmt.Errorf("rewrite pending event outbox entry: %w", err)
		}
		rewritten += result.ModifiedCount
	}
	if err := cursor.Err(); err != nil {
		return rewritten, fmt.Errorf("read pending event outbox entries: %w", err)
	}
	return rewritten, nil
}

func rewriteBodyKind(body []byte, from, to string) ([]byte, bool, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, fmt.Errorf("%w: decode envelope: %v", ErrInvalidEntry, err)
	}
	rawData, ok := envelope[fieldData]
	if !ok {
		return nil, false, nil
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(rawData, &data); err != nil {
		return nil, false, fmt.Errorf("%w: decode data: %v", ErrInvalidEntry, err)
	}
	rawKind, ok := data[fieldKind]
	if !ok {
		return nil, false, nil
	}
	var kind string
	if err := json.Unmarshal(rawKind, &kind); err != nil {
		return nil, false, fmt.Errorf("%w: decode data kind: %v", ErrInvalidEntry, err)
	}
	if kind != from {
		return nil, false, nil
	}
	encodedKind, err := json.Marshal(to)
	if err != nil {
		return nil, false, fmt.Errorf("%w: encode data kind: %v", ErrInvalidEntry, err)
	}
	data[fieldKind] = encodedKind
	encodedData, err := json.Marshal(data)
	if err != nil {
		return nil, false, fmt.Errorf("%w: encode data: %v", ErrInvalidEntry, err)
	}
	envelope[fieldData] = encodedData
	rewrittenBody, err := json.Marshal(envelope)
	if err != nil {
		return nil, false, fmt.Errorf("%w: encode envelope: %v", ErrInvalidEntry, err)
	}
	return rewrittenBody, true, nil
}
