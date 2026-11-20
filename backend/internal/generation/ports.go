package generation

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
)

type Store interface {
	Insert(ctx context.Context, doc Document) error
	ActiveByUser(ctx context.Context, userID string) (string, bool, error)
	FindOwned(ctx context.Context, userID, id string) (Document, error)
	MarkAccepted(ctx context.Context, userID, id, sheetID string) (bool, error)
	ReserveCap(ctx context.Context, dayUTC string, limit int, expiresAt time.Time) (used int, ok bool, err error)
}

type PublishStore interface {
	ClaimUnpublished(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Document, error)
	ReschedulePublish(ctx context.Context, id string, nextAt time.Time) error
	FailQueued(ctx context.Context, id, code string) (Document, bool, error)
	OldestQueued(ctx context.Context) (time.Time, bool, error)
}

type Publisher interface {
	Publish(ctx context.Context, msg Message) error
}

var ErrPublisherNotConfigured = errors.New("generation publisher is not configured")

type ConsentGate interface {
	Gate(ctx context.Context, userID string) error
}

type SheetService interface {
	CheckSheetLimit(ctx context.Context, ownerID string, kind plan.SheetKind) (sheet.LimitStatus, error)
	CreateWithValues(ctx context.Context, cmd sheet.CreateWithValuesCommand, afterInsert sheet.AfterInsertHook) (*sheet.Sheet, error)
}

type PlanSource interface {
	PlanOf(ctx context.Context, ownerID string) (plan.Plan, error)
}

type TemplateSource interface {
	FetchTemplate(ctx context.Context, cmd *template.FetchTemplateByIDCommand) (*template.Template, error)
}

type Quota interface {
	ReserveDaily(ctx context.Context, userID string) (string, error)
	RefundDaily(ctx context.Context, userID, dayUTC string) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type CapObserver interface {
	GenerationCapUsage(used, limit int)
}

type JobStore interface {
	ClaimQueued(ctx context.Context, id string, now time.Time, lease time.Duration) (Document, bool, error)
	Complete(ctx context.Context, id string, values map[string]any) (bool, error)
	FailRunning(ctx context.Context, id, code string) (Document, bool, error)
}

type LeaseStore interface {
	ClaimExpired(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Document, error)
	RescheduleExpired(ctx context.Context, id string, nextAt time.Time) error
	FailExpired(ctx context.Context, id string, now time.Time, code string) (Document, bool, error)
	OldestRunning(ctx context.Context) (time.Time, bool, error)
}

type Catalog interface {
	FetchByID(id string) (*template.Template, bool)
	DAG(id string) (*template.DAG, bool)
}
