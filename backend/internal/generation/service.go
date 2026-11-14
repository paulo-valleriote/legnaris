package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

// ErrNotFound reports a generation that is missing or belongs to another player.
// Both answer 404, so existence never leaks.
var ErrNotFound = errors.New("generation not found")

// ErrActiveExists reports an insert refused because the player already has a
// queued or running generation.
var ErrActiveExists = errors.New("generation already in progress")

// errNotAcceptable aborts an accept whose document left the succeeded state
// between the read and the write, for example to a concurrent accept.
var errNotAcceptable = errors.New("generation is not succeeded")

// Config is the static configuration of the generation flow.
type Config struct {
	// Enabled is the kill-switch: when false, no request reserves anything.
	Enabled bool
	// DailyCap is the global number of jobs accepted per UTC day.
	DailyCap int
}

// Deps are the collaborators of the service, each behind a port so the gate order
// is unit-tested on mocks.
type Deps struct {
	Store     Store
	Publisher Publisher
	Consent   ConsentGate
	Sheets    SheetService
	Plans     PlanSource
	Templates TemplateSource
	Quota     Quota
	Tx        Transactor
	Observer  CapObserver
	Logger    logger.StructuredLogger
}

// CreateCommand is POST /generations after the path and body are decoded.
type CreateCommand struct {
	UserID     string
	TemplateID string
	Kind       string
	Concept    string
}

// AcceptCommand is POST /generations/{id}/accept.
type AcceptCommand struct {
	UserID    string
	ID        string
	SheetName string
	Values    map[string]any
}

// AcceptResult reports the Sheet a generation became. Created is false when the
// generation was already accepted, so the caller answers 200 instead of 201.
type AcceptResult struct {
	SheetID string
	Created bool
}

// Service is the generation use case behind the /generations routes and the plan view.
type Service struct {
	cfg  Config
	deps Deps
	now  func() time.Time
}

// NewService builds the generation service. A nil now uses the wall clock.
func NewService(cfg Config, deps Deps, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{cfg: cfg, deps: deps, now: now}
}

// Create runs the gates of POST /generations in order and, when all pass, inserts
// the queued document with its cap and quota reservations in one transaction and
// publishes the job. The gates are: kill-switch, request shape, consent, Sheet
// limit (read-only), one job in progress, global daily cap, quota. A publish
// failure does not fail the request; the sweeper republishes the document.
func (s *Service) Create(ctx context.Context, cmd CreateCommand) (string, error) {
	if !s.cfg.Enabled {
		return "", apperr.NewError(apperr.CodeProviderUnavailable, MsgDisabled, nil)
	}
	if err := validateCreate(cmd); err != nil {
		return "", err
	}
	if err := s.requireTemplate(ctx, cmd.TemplateID); err != nil {
		return "", err
	}
	if err := s.deps.Consent.Gate(ctx, cmd.UserID); err != nil {
		return "", err
	}
	if err := s.requireSheetRoom(ctx, cmd.UserID); err != nil {
		return "", err
	}
	if err := s.requireNoJobInProgress(ctx, cmd.UserID); err != nil {
		return "", err
	}

	current, err := s.deps.Plans.PlanOf(ctx, cmd.UserID)
	if err != nil {
		return "", apperr.NewError(apperr.CodeInternal, MsgUnavailable, fmt.Errorf("resolve plan: %w", err))
	}

	id, err := idgen.New()
	if err != nil {
		return "", apperr.NewError(apperr.CodeInternal, MsgUnavailable, fmt.Errorf("generate id: %w", err))
	}
	now := s.now().UTC()
	doc := Document{
		ID:           id,
		UserID:       cmd.UserID,
		TemplateID:   cmd.TemplateID,
		Kind:         KindCharacter,
		Concept:      strings.TrimSpace(cmd.Concept),
		Status:       StatusQueued,
		ActiveUserID: cmd.UserID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(Retention),
	}

	if err := s.reserveAndInsert(ctx, &doc, current.ID == plan.IDPaid); err != nil {
		return "", err
	}
	s.publish(ctx, Message{ID: id, Kind: KindCharacter})
	return id, nil
}

// reserveAndInsert takes the global cap and the player's quota and inserts the
// document, all in one transaction, so a refused unit leaves no trace.
func (s *Service) reserveAndInsert(ctx context.Context, doc *Document, paid bool) error {
	limit := s.cfg.DailyCap
	var observedUsed *int
	err := s.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		day := dayKey(doc.CreatedAt)
		used, ok, err := s.deps.Store.ReserveCap(txCtx, day, limit, dayStart(doc.CreatedAt).Add(quotaRetention))
		if err != nil {
			return fmt.Errorf("reserve generation cap: %w", err)
		}
		observedUsed = &used
		if !ok {
			return apperr.NewError(apperr.CodeGenerationCapReached, MsgCapReached, nil).WithRetryAfter(untilNextDay(doc.CreatedAt))
		}

		if err := s.reserveQuota(txCtx, doc, paid); err != nil {
			return err
		}
		return s.deps.Store.Insert(txCtx, *doc)
	})
	if observedUsed != nil {
		s.deps.Observer.GenerationCapUsage(*observedUsed, limit)
	}
	return translateInsertErr(err)
}

// quotaRetention keeps a day's cap counter one more day, like the quota counter,
// so a request that straddles midnight still lands on its own day.
const quotaRetention = 48 * time.Hour

// reserveQuota takes the unit the player's plan draws from and records it on the
// document, so a later refund returns it to the same allowance.
func (s *Service) reserveQuota(ctx context.Context, doc *Document, paid bool) error {
	if paid {
		day, err := s.deps.Quota.ReserveDaily(ctx, doc.UserID)
		if err != nil {
			return err
		}
		doc.Quota = Reservation{Kind: ReservationDaily, DayUTC: day}
		return nil
	}
	if err := s.deps.Quota.ReserveTrial(ctx, doc.UserID); err != nil {
		return err
	}
	doc.Quota = Reservation{Kind: ReservationTrial}
	return nil
}

// translateInsertErr maps the transaction outcome. An in-progress collision
// becomes GENERATION_IN_PROGRESS; an apperr raised by a gate passes through.
func translateInsertErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrActiveExists) {
		return apperr.NewError(apperr.CodeGenerationInProgress, MsgInProgress, err)
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
}

// publish sends the job after the commit. A failure is logged with the id only,
// never the concept, and the republish sweep recovers the job.
func (s *Service) publish(ctx context.Context, msg Message) {
	if err := s.deps.Publisher.Publish(ctx, msg); err != nil {
		s.deps.Logger.Warn(ctx, "generation publish failed; the sweeper will republish",
			logger.Attribute{Key: "generation_id", Value: msg.ID},
			logger.Attribute{Key: "error", Value: err.Error()})
	}
}

// Fetch answers GET /generations/{id} for the player, or ErrNotFound-mapped 404.
func (s *Service) Fetch(ctx context.Context, userID, id string) (View, error) {
	if err := validateID(id); err != nil {
		return View{}, err
	}
	doc, err := s.deps.Store.FindOwned(ctx, userID, id)
	if err != nil {
		return View{}, translateFindErr(err)
	}
	return viewOf(doc), nil
}

// ActiveGenerationID reports the player's queued or running generation, for the
// plan view.
func (s *Service) ActiveGenerationID(ctx context.Context, userID string) (string, bool, error) {
	id, ok, err := s.deps.Store.ActiveByUser(ctx, userID)
	if err != nil {
		return "", false, apperr.NewError(apperr.CodeInternal, MsgUnavailable, fmt.Errorf("active generation: %w", err))
	}
	return id, ok, nil
}

// Accept turns a succeeded generation into a Sheet. The values are validated
// through the normal DAG path and the Sheet slot is reserved in the same
// transaction that marks the generation accepted. A repeat accept answers the
// same Sheet with Created false.
func (s *Service) Accept(ctx context.Context, cmd AcceptCommand) (AcceptResult, error) {
	if err := validateID(cmd.ID); err != nil {
		return AcceptResult{}, err
	}
	if strings.TrimSpace(cmd.SheetName) == "" {
		return AcceptResult{}, apperr.NewError(apperr.CodeInvalidRequest, MsgNameRequired, nil)
	}
	doc, err := s.deps.Store.FindOwned(ctx, cmd.UserID, cmd.ID)
	if err != nil {
		return AcceptResult{}, translateFindErr(err)
	}
	if doc.Status == StatusAccepted {
		return AcceptResult{SheetID: doc.SheetID}, nil
	}
	if doc.Status != StatusSucceeded {
		return AcceptResult{}, apperr.NewError(apperr.CodeConflict, MsgNotAcceptable, nil)
	}

	created, err := s.deps.Sheets.CreateWithValues(ctx, sheet.CreateWithValuesCommand{
		TemplateID: doc.TemplateID,
		Kind:       KindCharacter,
		SheetName:  strings.TrimSpace(cmd.SheetName),
		OwnerID:    cmd.UserID,
		Values:     cmd.Values,
	}, s.markAccepted(cmd.UserID, cmd.ID))
	if errors.Is(err, errNotAcceptable) {
		return s.resolveLostAccept(ctx, cmd)
	}
	if err != nil {
		return AcceptResult{}, translateAcceptErr(err)
	}
	return AcceptResult{SheetID: created.ID, Created: true}, nil
}

// markAccepted is the hook that runs inside the Sheet transaction. The conditional
// write means only one accept can win, and a loser rolls back its own Sheet.
func (s *Service) markAccepted(userID, id string) sheet.AfterInsertHook {
	return func(txCtx context.Context, created *sheet.Sheet) error {
		ok, err := s.deps.Store.MarkAccepted(txCtx, userID, id, created.ID)
		if err != nil {
			return fmt.Errorf("mark generation accepted: %w", err)
		}
		if !ok {
			return errNotAcceptable
		}
		return nil
	}
}

// resolveLostAccept answers a request that lost the accept race: when the winner
// already accepted the generation, the loser gets the winner's Sheet, which keeps
// accept idempotent under concurrency.
func (s *Service) resolveLostAccept(ctx context.Context, cmd AcceptCommand) (AcceptResult, error) {
	doc, err := s.deps.Store.FindOwned(ctx, cmd.UserID, cmd.ID)
	if err != nil {
		return AcceptResult{}, translateFindErr(err)
	}
	if doc.Status == StatusAccepted {
		return AcceptResult{SheetID: doc.SheetID}, nil
	}
	return AcceptResult{}, apperr.NewError(apperr.CodeConflict, MsgNotAcceptable, nil)
}

// translateAcceptErr passes an apperr from the sheet service through and wraps any
// other failure as an internal error.
func translateAcceptErr(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
}

// requireTemplate checks the template id and that it accepts characters, with the
// same validation as the template read.
func (s *Service) requireTemplate(ctx context.Context, templateID string) error {
	tmpl, err := s.deps.Templates.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: templateID})
	if err != nil {
		var appErr *apperr.Error
		if errors.As(err, &appErr) {
			return appErr
		}
		return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
	}
	if !tmpl.AcceptsKind(plan.KindCharacter) {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateKind, nil)
	}
	return nil
}

// requireSheetRoom is the read-only Sheet limit check: it never reserves a slot.
func (s *Service) requireSheetRoom(ctx context.Context, userID string) error {
	status, err := s.deps.Sheets.CheckSheetLimit(ctx, userID, plan.KindCharacter)
	if err != nil {
		return err
	}
	if status.Reached() {
		return apperr.NewError(apperr.CodeSheetLimitReached, sheet.MsgSheetLimitReached, nil).
			WithDetails(map[string]any{"kind": string(plan.KindCharacter), "limit": status.Limit})
	}
	return nil
}

// requireNoJobInProgress refuses a second job while one is queued or running. The
// sparse unique index still guards the race that the read cannot see.
func (s *Service) requireNoJobInProgress(ctx context.Context, userID string) error {
	_, active, err := s.deps.Store.ActiveByUser(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgUnavailable, fmt.Errorf("check generation in progress: %w", err))
	}
	if active {
		return apperr.NewError(apperr.CodeGenerationInProgress, MsgInProgress, nil)
	}
	return nil
}

// validateCreate checks the request shape before any gate reads or reserves.
func validateCreate(cmd CreateCommand) error {
	if cmd.UserID == "" {
		return apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	if cmd.Kind != KindCharacter {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgKindInvalid, nil)
	}
	length := utf8.RuneCountInString(strings.TrimSpace(cmd.Concept))
	if length < MinConceptLength || length > MaxConceptLength {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgConceptLength, nil)
	}
	return nil
}

// validateID rejects a malformed generation id before any read.
func validateID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgIDInvalid, err)
	}
	return nil
}

// translateFindErr maps a missing or foreign document to 404.
func translateFindErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperr.NewError(apperr.CodeNotFound, MsgNotFound, err)
	}
	return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
}

// viewOf maps the stored document to the read model, exposing values only once
// the job succeeded.
func viewOf(doc Document) View {
	view := View{
		ID:         doc.ID,
		Status:     doc.Status,
		Kind:       doc.Kind,
		TemplateID: doc.TemplateID,
		CreatedAt:  doc.CreatedAt,
	}
	if doc.Status == StatusSucceeded || doc.Status == StatusAccepted {
		view.Values = doc.Values
	}
	if doc.Status == StatusFailed {
		view.ErrorCode = doc.ErrorCode
	}
	return view
}

// dayKey is the UTC day a cap or quota counter is keyed by.
func dayKey(t time.Time) string {
	return dayStart(t).Format("2006-01-02")
}

// dayStart returns the UTC midnight that opens t's day.
func dayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// untilNextDay is the Retry-After of a refused cap: the wait to the next UTC
// midnight, when the counter starts over.
func untilNextDay(t time.Time) time.Duration {
	return dayStart(t).Add(24 * time.Hour).Sub(t.UTC())
}
