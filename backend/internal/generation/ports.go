package generation

import (
	"context"
	"time"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
)

// Store is the persistence port of the API: the generation documents and the
// global daily cap counter. Writes that belong to a reservation take the context
// of the transaction they run in.
type Store interface {
	// Insert stores a queued document. A second in-progress document for the same
	// player is ErrActiveExists.
	Insert(ctx context.Context, doc Document) error
	// ActiveByUser returns the id of the player's queued or running generation.
	ActiveByUser(ctx context.Context, userID string) (string, bool, error)
	// FindOwned returns the document when it belongs to userID; a missing or
	// foreign document is ErrNotFound.
	FindOwned(ctx context.Context, userID, id string) (Document, error)
	// MarkAccepted moves a succeeded document to accepted and records its Sheet. It
	// reports false when the document is not succeeded for userID any more.
	MarkAccepted(ctx context.Context, userID, id, sheetID string) (bool, error)
	// ReserveCap takes one job of the day's global cap while the day is below
	// limit. It returns the day's count after the attempt and whether it was taken.
	ReserveCap(ctx context.Context, dayUTC string, limit int, expiresAt time.Time) (used int, ok bool, err error)
}

// PublishStore is the persistence port of the republish sweep over queued
// documents.
type PublishStore interface {
	// ClaimUnpublished leases up to limit queued documents older than
	// RepublishAge whose previous lease has expired, and returns them.
	ClaimUnpublished(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Document, error)
	// ReschedulePublish counts one failed publish and makes the document eligible
	// again at nextAt.
	ReschedulePublish(ctx context.Context, id string, nextAt time.Time) error
	// FailQueued moves a queued document to failed with code. It returns the
	// document as it was before the move and false when it was not queued.
	FailQueued(ctx context.Context, id, code string) (Document, bool, error)
	// OldestQueued returns the creation time of the oldest queued document.
	OldestQueued(ctx context.Context) (time.Time, bool, error)
}

// Publisher publishes one job to the generation queue. An implementation waits for
// the broker's confirmation, so a nil return means the job is routed.
type Publisher interface {
	Publish(ctx context.Context, msg Message) error
}

// ConsentGate refuses unless the player holds a current LLM consent (ADR 0008).
// llmconsent.Service satisfies it.
type ConsentGate interface {
	Gate(ctx context.Context, userID string) error
}

// SheetService is the part of the sheet service the generation flow uses: the
// read-only limit check before enqueue and the accept-time creation.
type SheetService interface {
	CheckSheetLimit(ctx context.Context, ownerID string, kind plan.SheetKind) (sheet.LimitStatus, error)
	CreateWithValues(ctx context.Context, cmd sheet.CreateWithValuesCommand, afterInsert sheet.AfterInsertHook) (*sheet.Sheet, error)
}

// PlanSource resolves the plan of a player; paid players draw from the daily
// quota, Free players from the lifetime trial.
type PlanSource interface {
	PlanOf(ctx context.Context, ownerID string) (plan.Plan, error)
}

// TemplateSource resolves a Template by id, with the same validation GET
// /templates/:id applies.
type TemplateSource interface {
	FetchTemplate(ctx context.Context, cmd *template.FetchTemplateByIDCommand) (*template.Template, error)
}

// Quota is the allowance port: the lifetime trial and the paid daily counter. The
// quota service satisfies it; reservations take the transaction context.
type Quota interface {
	ReserveTrial(ctx context.Context, userID string) error
	RefundTrial(ctx context.Context, userID string) error
	ReserveDaily(ctx context.Context, userID string) (string, error)
	RefundDaily(ctx context.Context, userID, dayUTC string) error
}

// Transactor runs fn in one transaction. *db.Transactor satisfies it.
type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// CapObserver receives the global daily cap after each reservation attempt, so
// the 80% alert reads a current ratio.
type CapObserver interface {
	GenerationCapUsage(used, limit int)
}

// JobStore is the persistence port of the worker. ClaimQueued, Complete and
// FailRunning each move one document through a single transition and report false
// when the document was not in the state the transition needs, so a duplicate
// delivery or a removed document is a no-op.
type JobStore interface {
	// ClaimQueued moves a queued document to running under a lease until now+lease.
	ClaimQueued(ctx context.Context, id string, now time.Time, lease time.Duration) (Document, bool, error)
	// Complete stores the validated values of a running document and marks it succeeded.
	Complete(ctx context.Context, id string, values map[string]any) (bool, error)
	// FailRunning marks a running document failed with code and returns it as it was.
	FailRunning(ctx context.Context, id, code string) (Document, bool, error)
}

// LeaseStore is the persistence port of the lease reaper over running documents.
type LeaseStore interface {
	// ClaimExpired leases up to limit running documents whose worker lease has ended.
	ClaimExpired(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Document, error)
	// RescheduleExpired makes a document due for the reaper again at nextAt.
	RescheduleExpired(ctx context.Context, id string, nextAt time.Time) error
	// FailExpired fails a running document whose lease ended at or before now.
	FailExpired(ctx context.Context, id string, now time.Time, code string) (Document, bool, error)
	// OldestRunning returns the creation time of the oldest running document.
	OldestRunning(ctx context.Context) (time.Time, bool, error)
}

// Catalog resolves a code-defined Template and its rules by ID. *template.Registry
// satisfies it.
type Catalog interface {
	FetchByID(id string) (*template.Template, bool)
	DAG(id string) (*template.DAG, bool)
}
