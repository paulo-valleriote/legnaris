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

var ErrNotFound = errors.New("generation not found")

var ErrActiveExists = errors.New("generation already in progress")

var errNotAcceptable = errors.New("generation is not succeeded")

type Config struct {
	Enabled        bool
	DailyCap       int
	DraftTemplates bool
}

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

type CreateCommand struct {
	UserID     string
	TemplateID string
	Kind       string
	Concept    string
}

type AcceptCommand struct {
	UserID    string
	ID        string
	SheetName string
	Values    map[string]any
}

type AcceptResult struct {
	SheetID string
	Created bool
}

type Service struct {
	cfg  Config
	deps Deps
	now  func() time.Time
}

func NewService(cfg Config, deps Deps, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{cfg: cfg, deps: deps, now: now}
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (string, error) {
	if !s.cfg.Enabled {
		return "", apperr.NewError(apperr.CodeProviderUnavailable, MsgDisabled, nil)
	}
	kind, err := validateCreate(cmd)
	if err != nil {
		return "", err
	}
	if err := s.requireTemplate(ctx, cmd.TemplateID, kind); err != nil {
		return "", err
	}
	current, err := s.deps.Plans.PlanOf(ctx, cmd.UserID)
	if err != nil {
		return "", apperr.NewError(apperr.CodeInternal, MsgUnavailable, fmt.Errorf("resolve plan: %w", err))
	}
	if current.ID != plan.IDPaid {
		return "", apperr.NewError(apperr.CodePlanUpgradeRequired, MsgPlanUpgradeRequired, nil)
	}
	if err := s.deps.Consent.Gate(ctx, cmd.UserID); err != nil {
		return "", err
	}
	if err := s.requireSheetRoom(ctx, cmd.UserID, kind); err != nil {
		return "", err
	}
	if err := s.requireNoJobInProgress(ctx, cmd.UserID); err != nil {
		return "", err
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
		Kind:         string(kind),
		Concept:      SanitizeConcept(cmd.Concept),
		Status:       StatusQueued,
		ActiveUserID: cmd.UserID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(Retention),
	}

	if err := s.reserveAndInsert(ctx, &doc); err != nil {
		return "", err
	}
	s.publish(ctx, Message{ID: id, Kind: string(kind)})
	return id, nil
}

func (s *Service) reserveAndInsert(ctx context.Context, doc *Document) error {
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

		if err := s.reserveQuota(txCtx, doc); err != nil {
			return err
		}
		return s.deps.Store.Insert(txCtx, *doc)
	})
	if observedUsed != nil {
		s.deps.Observer.GenerationCapUsage(*observedUsed, limit)
	}
	return translateInsertErr(err)
}

const quotaRetention = 48 * time.Hour

func (s *Service) reserveQuota(ctx context.Context, doc *Document) error {
	day, err := s.deps.Quota.ReserveDaily(ctx, doc.UserID)
	if err != nil {
		return err
	}
	doc.Quota = Reservation{Kind: ReservationDaily, DayUTC: day}
	return nil
}

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

func (s *Service) publish(ctx context.Context, msg Message) {
	if err := s.deps.Publisher.Publish(ctx, msg); err != nil {
		s.deps.Logger.Warn(ctx, "generation publish failed; the sweeper will republish",
			logger.Attribute{Key: "generation_id", Value: msg.ID},
			logger.Attribute{Key: "error", Value: err.Error()})
	}
}

func (s *Service) Fetch(ctx context.Context, userID, id string) (View, error) {
	if err := validateID(id); err != nil {
		return View{}, err
	}
	doc, err := s.deps.Store.FindOwned(ctx, userID, id)
	if err != nil {
		return View{}, translateFindErr(err)
	}
	view := viewOf(doc)
	view.SuggestedName = s.suggestedName(ctx, doc)
	return view, nil
}

func (s *Service) suggestedName(ctx context.Context, doc Document) *string {
	if doc.Status != StatusSucceeded {
		return nil
	}
	tmpl, err := s.deps.Templates.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: doc.TemplateID})
	if err != nil {
		s.deps.Logger.Warn(ctx, "generation name suggestion skipped; template unreadable",
			logger.Attribute{Key: "generation_id", Value: doc.ID},
			logger.Attribute{Key: "error", Value: err.Error()})
		return nil
	}
	name := tmpl.SuggestName(doc.Values)
	if name == "" {
		return nil
	}
	return &name
}

func (s *Service) ActiveGenerationID(ctx context.Context, userID string) (string, bool, error) {
	id, ok, err := s.deps.Store.ActiveByUser(ctx, userID)
	if err != nil {
		return "", false, apperr.NewError(apperr.CodeInternal, MsgUnavailable, fmt.Errorf("active generation: %w", err))
	}
	return id, ok, nil
}

func (s *Service) Accept(ctx context.Context, cmd AcceptCommand) (AcceptResult, error) {
	if err := validateID(cmd.ID); err != nil {
		return AcceptResult{}, err
	}
	if strings.TrimSpace(cmd.SheetName) == "" {
		return AcceptResult{}, apperr.NewError(apperr.CodeInvalidRequest, MsgNameRequired, nil)
	}
	if len(cmd.Values) == 0 {
		return AcceptResult{}, apperr.NewError(apperr.CodeInvalidRequest, MsgValuesRequired, nil)
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
		Kind:       documentKind(doc.Kind),
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

func translateAcceptErr(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
}

func (s *Service) requireTemplate(ctx context.Context, templateID string, kind plan.SheetKind) error {
	tmpl, err := s.deps.Templates.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: templateID})
	if err != nil {
		var appErr *apperr.Error
		if errors.As(err, &appErr) {
			return appErr
		}
		return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
	}
	if !tmpl.Creatable(s.cfg.DraftTemplates) {
		if tmpl.State == template.StateRetired {
			return apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateRetired, nil)
		}
		return apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateState, nil)
	}
	if !tmpl.AcceptsKind(kind) {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateKind, nil)
	}
	return nil
}

func (s *Service) requireSheetRoom(ctx context.Context, userID string, kind plan.SheetKind) error {
	status, err := s.deps.Sheets.CheckSheetLimit(ctx, userID, kind)
	if err != nil {
		return err
	}
	if status.Reached() {
		return apperr.NewError(apperr.CodeSheetLimitReached, sheet.MsgSheetLimitReached, nil).
			WithDetails(map[string]any{"kind": string(kind), "limit": status.Limit})
	}
	return nil
}

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

func validateCreate(cmd CreateCommand) (plan.SheetKind, error) {
	if cmd.UserID == "" {
		return "", apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	kind, ok := plan.ParseSheetKind(cmd.Kind)
	if !ok {
		return "", apperr.NewError(apperr.CodeInvalidRequest, MsgKindInvalid, nil)
	}
	length := utf8.RuneCountInString(SanitizeConcept(cmd.Concept))
	if length < MinConceptLength || length > MaxConceptLength {
		return "", apperr.NewError(apperr.CodeInvalidRequest, MsgConceptLength, nil)
	}
	return kind, nil
}

func validateID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgIDInvalid, err)
	}
	return nil
}

func translateFindErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperr.NewError(apperr.CodeNotFound, MsgNotFound, err)
	}
	return apperr.NewError(apperr.CodeInternal, MsgUnavailable, err)
}

func viewOf(doc Document) View {
	view := View{
		ID:         doc.ID,
		Status:     doc.Status,
		Kind:       documentKind(doc.Kind),
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

func documentKind(raw string) string {
	if raw == "" {
		return string(plan.KindCharacter)
	}
	return raw
}

func dayKey(t time.Time) string {
	return dayStart(t).Format("2006-01-02")
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func untilNextDay(t time.Time) time.Duration {
	return dayStart(t).Add(24 * time.Hour).Sub(t.UTC())
}
