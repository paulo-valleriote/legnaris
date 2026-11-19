package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	sharedjson "legnaris/backend/shared/json"
)

const (
	fieldEventType = "event_type"
	fieldBody      = "body"
	fieldData      = "data"
	fieldKind      = "kind"
	operandIn      = "$in"
)

var (
	ErrInvalidRewrite    = errors.New("invalid pending data kind rewrite")
	ErrOutboxLeased      = errors.New("event outbox is leased by a running relay")
	ErrRewriteConflict   = errors.New("pending event outbox entry changed during the rewrite")
	ErrRewriteIncomplete = errors.New("pending event outbox entries still carry the old kind")
)

func RewritePendingDataKind(ctx context.Context, database *mongo.Database, eventTypes []string, from, to string, now time.Time) (int64, error) {
	if err := validateRewrite(eventTypes, from, to); err != nil {
		return 0, err
	}

	collection := database.Collection(eventOutboxCollection)
	entries, err := pendingEntriesOfTypes(ctx, collection, eventTypes)
	if err != nil {
		return 0, err
	}
	if err := requireUnleased(entries, now); err != nil {
		return 0, err
	}

	var rewritten int64
	for _, entry := range entries {
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
		if result.MatchedCount == 0 {
			return rewritten, fmt.Errorf("%w: entry %s is no longer pending or changed its body", ErrRewriteConflict, entry.ID)
		}
		rewritten += result.ModifiedCount
	}

	if err := verifyNoOldKind(ctx, collection, eventTypes, from, to); err != nil {
		return rewritten, err
	}
	return rewritten, nil
}

func validateRewrite(eventTypes []string, from, to string) error {
	if len(eventTypes) == 0 {
		return fmt.Errorf("%w: no event type given", ErrInvalidRewrite)
	}
	if from == "" {
		return fmt.Errorf("%w: empty source kind", ErrInvalidRewrite)
	}
	if to == "" {
		return fmt.Errorf("%w: empty target kind", ErrInvalidRewrite)
	}
	if from == to {
		return fmt.Errorf("%w: source and target kinds are equal", ErrInvalidRewrite)
	}
	return nil
}

func pendingEntriesOfTypes(ctx context.Context, collection *mongo.Collection, eventTypes []string) ([]Entry, error) {
	cursor, err := collection.Find(ctx, bson.D{
		{Key: fieldStatus, Value: StatusPending},
		{Key: fieldEventType, Value: bson.D{{Key: operandIn, Value: eventTypes}}},
	})
	if err != nil {
		return nil, fmt.Errorf("scan pending event outbox entries: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var entries []Entry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("read pending event outbox entries: %w", err)
	}
	return entries, nil
}

func requireUnleased(entries []Entry, now time.Time) error {
	for _, entry := range entries {
		if entry.LockedUntil != nil && entry.LockedUntil.After(now) {
			return fmt.Errorf("%w: entry %s is leased until %s, stop the relay before rewriting",
				ErrOutboxLeased, entry.ID, entry.LockedUntil.UTC().Format(time.RFC3339))
		}
	}
	return nil
}

func verifyNoOldKind(ctx context.Context, collection *mongo.Collection, eventTypes []string, from, to string) error {
	entries, err := pendingEntriesOfTypes(ctx, collection, eventTypes)
	if err != nil {
		return err
	}

	var remaining int
	for _, entry := range entries {
		_, changed, err := rewriteBodyKind(entry.Body, from, to)
		if err != nil {
			return err
		}
		if changed {
			remaining++
		}
	}
	if remaining > 0 {
		return fmt.Errorf("%w: %d pending entries still carry %q", ErrRewriteIncomplete, remaining, from)
	}
	return nil
}

func rewriteBodyKind(body []byte, from, to string) ([]byte, bool, error) {
	var envelope map[string]json.RawMessage
	if err := sharedjson.DecodeBytes(body, &envelope); err != nil {
		return nil, false, fmt.Errorf("%w: decode envelope: %w", ErrInvalidEntry, err)
	}
	rawData, ok := envelope[fieldData]
	if !ok {
		return nil, false, nil
	}
	var data map[string]json.RawMessage
	if err := sharedjson.DecodeBytes(rawData, &data); err != nil {
		return nil, false, fmt.Errorf("%w: decode data: %w", ErrInvalidEntry, err)
	}
	rawKind, ok := data[fieldKind]
	if !ok {
		return nil, false, nil
	}
	var kind string
	if err := sharedjson.DecodeBytes(rawKind, &kind); err != nil {
		return nil, false, fmt.Errorf("%w: decode data kind: %w", ErrInvalidEntry, err)
	}
	if kind != from {
		return nil, false, nil
	}
	encodedKind, err := json.Marshal(to)
	if err != nil {
		return nil, false, fmt.Errorf("%w: encode data kind: %w", ErrInvalidEntry, err)
	}
	data[fieldKind] = encodedKind
	encodedData, err := json.Marshal(data)
	if err != nil {
		return nil, false, fmt.Errorf("%w: encode data: %w", ErrInvalidEntry, err)
	}
	envelope[fieldData] = encodedData
	rewrittenBody, err := json.Marshal(envelope)
	if err != nil {
		return nil, false, fmt.Errorf("%w: encode envelope: %w", ErrInvalidEntry, err)
	}
	return rewrittenBody, true, nil
}
