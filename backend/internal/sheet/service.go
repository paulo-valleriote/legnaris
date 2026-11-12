package sheet

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
	"legnaris/backend/shared/maputils"
)

type CreateSheetCommand struct {
	TemplateID string `json:"template_id"`
	SheetName  string `json:"sheet_name"`
	OwnerID    string `json:"-"`
}

// sheetRef names one sheet as seen by one owner: the pair every read-then-write
// path checks and fetches by.
type sheetRef struct {
	id      string
	ownerID string
}

// validate rejects a missing owner before a malformed sheet ID, the order every
// sheet path reports them in.
func (r sheetRef) validate() error {
	if err := requireOwner(r.ownerID); err != nil {
		return err
	}
	return requireSheetID(r.id)
}

// PartialUpdateSheetCommand carries one sheet save: an optional data delta and
// an optional full replacement of the Dice block. A nil Dice leaves the stored
// block untouched; a non-nil empty Dice clears every override. At least one of
// the two must be present. Preview reads Data only.
type PartialUpdateSheetCommand struct {
	SheetID   string         `json:"-"`
	OwnerID   string         `json:"-"`
	RequestID string         `json:"-"`
	Data      map[string]any `json:"data"`
	Dice      *Dice          `json:"dice"`
}

func (c PartialUpdateSheetCommand) ref() sheetRef {
	return sheetRef{id: c.SheetID, ownerID: c.OwnerID}
}

// hasDelta reports whether the save carries data to recalculate and write; a
// save without one is dice-only.
func (c PartialUpdateSheetCommand) hasDelta() bool {
	return len(c.Data) > 0
}

// SetLayoutCommand carries the full replacement of a sheet's Layout. A nil or
// empty Layout resets it to the Template default.
type SetLayoutCommand struct {
	SheetID string  `json:"-"`
	OwnerID string  `json:"-"`
	Layout  *Layout `json:"layout"`
}

// PreviewTemplateCommand carries the template dry-run input. TemplateID is
// filled from the path param (json:"-" keeps a body template_id out), so the
// body is the same {"data": <delta>} shape the sheet endpoints take.
type PreviewTemplateCommand struct {
	TemplateID string         `json:"-"`
	Data       map[string]any `json:"data"`
}

// DeleteSheetCommand carries the target sheet and the authenticated owner. Both
// come from the request (path param and JWT context), never from the body.
type DeleteSheetCommand struct {
	SheetID string `json:"-"`
	OwnerID string `json:"-"`
}

type Service interface {
	Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error)
	FetchMySheet(ctx context.Context, ownerID string) (*Sheet, error)
	FetchByID(ctx context.Context, id string, ownerID string) (*Sheet, error)
	PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
	Preview(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
	PreviewTemplate(ctx context.Context, cmd PreviewTemplateCommand) (map[string]any, error)
	SetLayout(ctx context.Context, cmd SetLayoutCommand) (*Layout, error)
	Delete(ctx context.Context, cmd DeleteSheetCommand) error
}

// User-facing messages for the errors this service builds itself. They are
// plain strings, never error causes — causes stay apperr-only per AGENTS.md.
// They stay exported so any boundary that needs the wording reads it from
// here, while the checks themselves have a single source in this package:
// controllers forward input to the service instead of repeating the
// validation (LGN-66).
const (
	MsgSheetIDNotProvided = "sheet id not provided"
	MsgSheetIDInvalid     = "sheet id must be a canonical UUID v7"
	MsgFieldDoesNotExist  = "field does not exist: "
	MsgNestedTooDeep      = "sheet data is nested too deeply"
	MsgOwnerNotProvided   = "authenticated user is required"
	MsgSheetLimitReached  = "user already owns a sheet"
	MsgSheetConflict      = "sheet was changed by another request, try again"
	MsgNothingToUpdate    = "sheet update carries neither data nor dice"
)

// conflictRetryAfter is the Retry-After hint sent with a PATCH that lost the
// compare-and-swap. Retrying is the client's job, so every retry is a new HTTP
// request and passes through the rate limiter.
const conflictRetryAfter = time.Second

// requireSheetID is the single source of truth for the sheet-ID checks
// (LGN-86): the missing-ID message stays byte-for-byte for existing callers,
// then the idgen contract rejects anything that is not a canonical UUID v7 —
// so junk dies with a 400 here instead of becoming a 404 repository lookup,
// no matter which handler or caller reached the service.
func requireSheetID(id string) error {
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDNotProvided, nil)
	}
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDInvalid, err)
	}
	return nil
}

// requireOwner rejects a request whose context carried no authenticated user
// id (OWN-19). The JWT middleware guarantees it on protected routes; failing
// closed here means a route mounted without the middleware can never act
// ownerless instead of silently operating on someone else's data.
func requireOwner(ownerID string) error {
	if ownerID == "" {
		return apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}
	return nil
}

// checkOwnership returns the same 404 a missing sheet produces when the stored
// sheet belongs to nobody (legacy row) or to a different user, so a foreign
// sheet is indistinguishable from a nonexistent one and leaks no existence
// (OWN-11, OWN-12, OWN-24).
func checkOwnership(stored *Sheet, ownerID string) error {
	if stored.OwnerID == "" || stored.OwnerID != ownerID {
		return translateRepoErr("fetch", ErrSheetNotFound)
	}
	return nil
}

// DAGProvider resolves the compiled DAG for a template ID. It is satisfied by
// *template.Registry, which is built once at boot and never mutated afterwards:
// the sheet service reads DAGs through that read-only view instead of holding a
// map it could mutate or that could mutate underneath it (LGN-54).
type DAGProvider interface {
	DAG(id string) (*template.DAG, bool)
}

type ServiceImpl struct {
	repository      Repository
	templateService template.Service
	provider        DAGProvider
	log             logger.StructuredLogger
}

// NewService builds the sheet service over a DAG provider. The provider must
// already be complete when the service starts (boot-time registry): neither the
// service nor its callers mutate it afterwards.
func NewService(repository Repository, templateService template.Service, provider DAGProvider, log logger.StructuredLogger) Service {
	return &ServiceImpl{
		repository:      repository,
		templateService: templateService,
		provider:        provider,
		log:             log,
	}
}

func (s *ServiceImpl) Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error) {
	// Fail closed before any repository access: a route mounted without the JWT
	// middleware must never create an ownerless sheet (OWN-19).
	if cmd.OwnerID == "" {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgOwnerNotProvided, nil)
	}

	// FetchTemplate before the DAG lookup: it owns the template-ID checks
	// (missing, malformed slug), so POST /sheets fails exactly like
	// GET /templates/:id instead of collapsing every bad ID into a 404 (LGN-86).
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: cmd.TemplateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}

	dag, exists := s.provider.DAG(cmd.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	// Best-effort pre-check for a friendly 409 on the common path. The unique
	// sheets.owner_id index remains the authoritative guard: only the insert
	// closes the concurrent-create race (OWN-03, OWN-04).
	if _, err := s.repository.FetchByOwnerID(ctx, cmd.OwnerID); err == nil {
		return nil, apperr.NewError(apperr.CodeSheetLimitReached, MsgSheetLimitReached, nil)
	} else if !errors.Is(err, ErrSheetNotFound) {
		return nil, translateRepoErr("create", err)
	}

	initialData := buildNestedAllFields(tmpl.Fields)

	// Flatten the initial data into dot-notation keys so the DAG can match
	// leaf keys during recalc (the DAG walks leaf keys, not top-level groups).
	flatInitial, err := maputils.FlattenPathKeys(initialData)
	if err != nil {
		return nil, translateMapErr("create", err)
	}

	derivedEffects, err := dag.Recalculate(map[string]any{}, flatInitial)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	// Apply the flat derived effects back as a nested map (same strategy as
	// PartialUpdate), instead of maps.Copy which would leak flat keys.
	if err := maputils.ApplyUpdates(initialData, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}

	sheetID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, "failed to generate sheet id", err)
	}

	newSheet := Sheet{
		ID:         sheetID,
		TemplateID: cmd.TemplateID,
		OwnerID:    cmd.OwnerID,
		Name:       cmd.SheetName,
		Data:       initialData,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repository.Create(ctx, &newSheet); err != nil {
		return nil, translateRepoErr("create", err)
	}
	return &newSheet, nil
}

// FetchMySheet returns the single sheet owned by ownerID, or 404 NOT_FOUND when
// the owner has none (OWN-07, OWN-08).
func (s *ServiceImpl) FetchMySheet(ctx context.Context, ownerID string) (*Sheet, error) {
	if err := requireOwner(ownerID); err != nil {
		return nil, err
	}

	sheet, err := s.repository.FetchByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	return sheet, nil
}

func (s *ServiceImpl) FetchByID(ctx context.Context, id string, ownerID string) (*Sheet, error) {
	if err := requireOwner(ownerID); err != nil {
		return nil, err
	}
	if err := requireSheetID(id); err != nil {
		return nil, err
	}

	sheet, err := s.repository.FetchByID(ctx, id)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}

	if err := checkOwnership(sheet, ownerID); err != nil {
		return nil, err
	}

	return sheet, nil
}

// preparedSave is what a save would write, resolved against the stored sheet
// without persisting it: the version the compare-and-swap expects, the merged
// data a write would store and the flat cascading effects the DAG recomputed.
// A dice-only save carries an empty delta and cascade and no merged data.
// Persisting callers use merged; previews use cascade only.
type preparedSave struct {
	templateID string
	version    int64
	delta      map[string]any
	merged     map[string]any
	cascade    map[string]any
}

// recalculate is the one recalc path PATCH and both previews share: it
// requires the sheet ID, fetches the sheet, resolves the DAG, flattens the
// delta, normalizes and recalculates it, then deep-merges the result over the
// stored data. It performs no repository write, so a preview can call it
// without side effects.
func (s *ServiceImpl) recalculate(ctx context.Context, ref sheetRef, data map[string]any) (*preparedSave, error) {
	storedSheet, err := s.fetchOwned(ctx, ref)
	if err != nil {
		return nil, err
	}

	dag, exists := s.provider.DAG(storedSheet.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "DAG not found for template", nil)
	}

	// The API contract is a nested map ({"data":{"stats":{"dexterity":12}}}).
	// Flatten it to flat path keys because the DAG engine operates
	// exclusively on those. A payload nested deeper than
	// maputils.MaxNestingDepth is rejected here, before the recursive walk can
	// exhaust the stack.
	updates, err := maputils.FlattenPathKeys(data)
	if err != nil {
		return nil, translateMapErr("update", err)
	}

	// Validate editability, collapse each touched exclusive group to at most
	// one non-zero member, and recompute derived fields under a single DAG
	// read-lock acquisition, so a request never takes the lock more than once.
	normalized, cascadingEffects, err := dag.NormalizeAndRecalculate(storedSheet.Data, updates)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	mergedData := maputils.DeepCopy(storedSheet.Data)
	if err := maputils.ApplyUpdates(mergedData, normalized); err != nil {
		return nil, translateMapErr("update", err)
	}
	if err := maputils.ApplyUpdates(mergedData, cascadingEffects); err != nil {
		return nil, translateMapErr("update", err)
	}

	return &preparedSave{
		templateID: storedSheet.TemplateID,
		version:    storedSheet.Version,
		delta:      normalized,
		merged:     mergedData,
		cascade:    cascadingEffects,
	}, nil
}

// fetchOwned requires the owner and the sheet ID, fetches the sheet and checks
// ownership, so every read-then-write path rejects the same inputs the same way.
func (s *ServiceImpl) fetchOwned(ctx context.Context, ref sheetRef) (*Sheet, error) {
	if err := ref.validate(); err != nil {
		return nil, err
	}

	storedSheet, err := s.repository.FetchByID(ctx, ref.id)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}
	if err := checkOwnership(storedSheet, ref.ownerID); err != nil {
		return nil, err
	}
	return storedSheet, nil
}

// PartialUpdate saves the delta and the Dice block together under one
// compare-and-swap on the sheet version (ADR 0006), so a refused block never
// leaves a half-written save behind. A data delta is recalculated; a save that
// carries only dice skips the DAG and returns an empty cascade. A lost race is
// never retried here: it returns CONFLICT with a Retry-After hint and the
// client resends, so the retry is a new request that the rate limiter accounts
// for.
func (s *ServiceImpl) PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	result, err := s.prepareSave(ctx, cmd)
	if err != nil {
		return nil, err
	}

	dbUpdates := map[string]any{"updated_at": time.Now()}
	if cmd.hasDelta() {
		dbUpdates["data"] = result.merged
	}
	if cmd.Dice != nil {
		block, err := s.validatedDice(ctx, result.templateID, cmd.Dice)
		if err != nil {
			return nil, err
		}
		dbUpdates[diceField] = block
	}

	newVersion, err := s.repository.PartialUpdate(ctx, cmd.SheetID, result.version, dbUpdates)
	if err == nil {
		s.log.Info(ctx, "sheet patched", append(
			patchLogFields(cmd.SheetID, cmd.RequestID, result.templateID, result.version, result.delta, result.cascade),
			logger.Attribute{Key: "dice_replaced", Value: strconv.FormatBool(cmd.Dice != nil)},
			logger.Attribute{Key: "version_after", Value: strconv.FormatInt(newVersion, 10)},
		)...)
		return nestedCascadingEffects(result.cascade)
	}
	if !errors.Is(err, ErrVersionConflict) {
		return nil, translateRepoErr("update", err)
	}
	s.log.Warn(ctx, "sheet patch version conflict",
		patchLogFields(cmd.SheetID, cmd.RequestID, result.templateID, result.version, result.delta, result.cascade)...)
	return nil, apperr.NewError(apperr.CodeConflict, MsgSheetConflict, ErrVersionConflict).
		WithRetryAfter(conflictRetryAfter)
}

// prepareSave resolves what a save writes: the recalculation of a data delta,
// or, for a dice-only save, the stored sheet's version with an empty delta and
// cascade so the DAG is never consulted. A save carrying neither is refused
// after the owner and ID checks and before any repository access.
func (s *ServiceImpl) prepareSave(ctx context.Context, cmd PartialUpdateSheetCommand) (*preparedSave, error) {
	if cmd.hasDelta() {
		return s.recalculate(ctx, cmd.ref(), cmd.Data)
	}
	if err := cmd.ref().validate(); err != nil {
		return nil, err
	}
	if cmd.Dice == nil {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgNothingToUpdate, nil)
	}
	storedSheet, err := s.fetchOwned(ctx, cmd.ref())
	if err != nil {
		return nil, err
	}
	return &preparedSave{
		templateID: storedSheet.TemplateID,
		version:    storedSheet.Version,
		delta:      map[string]any{},
		cascade:    map[string]any{},
	}, nil
}

// patchLogFields builds the attributes shared by every PATCH record. Values are
// included only for numeric leaves: string values are player free text (names,
// notes) and must never reach the log, so those leaves are identified by path.
func patchLogFields(sheetID, requestID, templateID string, versionBefore int64, delta, cascade map[string]any) []logger.Attribute {
	return []logger.Attribute{
		{Key: "sheet_id", Value: sheetID},
		{Key: "template_id", Value: templateID},
		{Key: "request_id", Value: requestID},
		{Key: "version_before", Value: strconv.FormatInt(versionBefore, 10)},
		{Key: "delta_paths", Value: joinSortedPaths(delta)},
		{Key: "delta_values", Value: numericValuesOf(delta)},
		{Key: "cascade_paths", Value: joinSortedPaths(cascade)},
		{Key: "cascade_values", Value: numericValuesOf(cascade)},
	}
}

// joinSortedPaths lists the flat path keys in sorted order, so the same write
// always produces the same log line.
func joinSortedPaths(flat map[string]any) string {
	paths := make([]string, 0, len(flat))
	for path := range flat {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return strings.Join(paths, ",")
}

// numericValuesOf renders the numeric leaves of a flat map as sorted
// "path=value" pairs. Any non-numeric leaf is skipped, not rendered.
func numericValuesOf(flat map[string]any) string {
	pairs := make([]string, 0, len(flat))
	for path, value := range flat {
		if text, ok := numericText(value); ok {
			pairs = append(pairs, path+"="+text)
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

// numericText formats a numeric leaf for the log. Only float64 and int are
// handled: JSON decoding yields float64 and integer arithmetic in expr-lang
// yields int. The second result is false for every other type, which keeps the
// value out of the log.
func numericText(value any) (string, bool) {
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	}
	return "", false
}

// Preview runs the same recalculation as PartialUpdate and returns the
// cascading effects, persisting nothing: it shares recalculate, so the cascade
// matches an equivalent PATCH while the stored sheet and updated_at stay
// untouched. Repeated calls against an unchanged sheet return the same map.
func (s *ServiceImpl) Preview(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	result, err := s.recalculate(ctx, cmd.ref(), cmd.Data)
	if err != nil {
		return nil, err
	}
	return nestedCascadingEffects(result.cascade)
}

// PreviewTemplate dry-runs a delta against a template with no stored sheet. It
// seeds the template exactly like Create (every leaf at its SeedValue, derived
// leaves then filled by the DAG), applies the delta through the same
// NormalizeAndRecalculate the PATCH path uses and returns the nested cascade.
// Nothing is persisted: there is no sheet to write.
func (s *ServiceImpl) PreviewTemplate(ctx context.Context, cmd PreviewTemplateCommand) (map[string]any, error) {
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: cmd.TemplateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}

	dag, exists := s.provider.DAG(cmd.TemplateID)
	if !exists {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	seeded := buildNestedAllFields(tmpl.Fields)

	flatSeed, err := maputils.FlattenPathKeys(seeded)
	if err != nil {
		return nil, translateMapErr("create", err)
	}

	derivedEffects, err := dag.Recalculate(map[string]any{}, flatSeed)
	if err != nil {
		return nil, translateDAGErr(err)
	}
	if err := maputils.ApplyUpdates(seeded, derivedEffects); err != nil {
		return nil, translateMapErr("create", err)
	}

	updates, err := maputils.FlattenPathKeys(cmd.Data)
	if err != nil {
		return nil, translateMapErr("update", err)
	}

	_, cascading, err := dag.NormalizeAndRecalculate(seeded, updates)
	if err != nil {
		return nil, translateDAGErr(err)
	}

	return nestedCascadingEffects(cascading)
}

// SetLayout replaces the owner's sheet Layout after checking every key against
// the Rows of the sheet's Template. It is idempotent and writes only the layout
// field: no recalculation runs and the DAG is never consulted, because ordering
// is not a value. An empty Layout is stored as none (reset to default); a
// Layout identical to the Template order is stored as given.
func (s *ServiceImpl) SetLayout(ctx context.Context, cmd SetLayoutCommand) (*Layout, error) {
	if err := requireOwner(cmd.OwnerID); err != nil {
		return nil, err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return nil, err
	}

	stored, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return nil, translateRepoErr("fetch", err)
	}
	if err := checkOwnership(stored, cmd.OwnerID); err != nil {
		return nil, err
	}

	var layout *Layout
	if !cmd.Layout.IsEmpty() {
		tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: stored.TemplateID})
		if err != nil {
			return nil, translateTemplateErr(err)
		}
		if err := template.ValidateLayout(*tmpl, cmd.Layout.SectionOrder, cmd.Layout.Pinned); err != nil {
			return nil, translateLayoutErr(err)
		}
		layout = cmd.Layout.Clone()
	}

	if err := s.repository.UpdateLayout(ctx, cmd.SheetID, layout); err != nil {
		return nil, translateRepoErr("update", err)
	}
	return layout, nil
}

// validatedDice checks a Dice block against the sheet's Template and returns
// what to store. An empty block clears every override and is stored as none,
// without consulting the Template; a null entry is a choice ("no dice") and is
// stored as given. The returned block is a copy, so it never aliases the command.
func (s *ServiceImpl) validatedDice(ctx context.Context, templateID string, dice *Dice) (*Dice, error) {
	if dice.IsEmpty() {
		return nil, nil
	}
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: templateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}
	if err := template.ValidateSheetDice(*tmpl, dice.Section, dice.Row); err != nil {
		return nil, translateDiceErr(err)
	}
	return dice.Clone(), nil
}

// Delete removes the owner's sheet. Ownership is verified first, so a foreign
// sheet and a missing sheet both return the repository's 404 without deleting
// anything, and a second delete is 404 because the fetch misses (OWN-13,
// OWN-23).
func (s *ServiceImpl) Delete(ctx context.Context, cmd DeleteSheetCommand) error {
	if err := requireOwner(cmd.OwnerID); err != nil {
		return err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return err
	}

	stored, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return translateRepoErr("delete", err)
	}
	if err := checkOwnership(stored, cmd.OwnerID); err != nil {
		return err
	}

	if err := s.repository.DeleteByID(ctx, cmd.SheetID); err != nil {
		return translateRepoErr("delete", err)
	}
	return nil
}

// translateTemplateErr maps template-layer typed errors to apperr at the
// service boundary. The template layer must stay free of apperr imports for
// its own sentinels; the /templates controller runs the same translation
// through templateError.
func translateTemplateErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, template.ErrTemplateIDNotProvided) || errors.Is(err, template.ErrTemplateIDInvalid) {
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to fetch template", err)
}

// translateLayoutErr maps template layout validation errors to a 400
// INVALID_REQUEST at the service boundary. The template layer must stay free of
// apperr imports.
func translateLayoutErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	var unknownSection *template.UnknownLayoutSectionError
	var unknownRow *template.UnknownLayoutRowError
	var duplicate *template.DuplicateLayoutKeyError
	if errors.As(err, &unknownSection) || errors.As(err, &unknownRow) || errors.As(err, &duplicate) {
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid layout: "+err.Error(), err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to validate layout", err)
}

// translateDiceErr maps template dice validation errors to a 400
// INVALID_REQUEST at the service boundary. The template layer must stay free of
// apperr imports.
func translateDiceErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	var unknownSection *template.UnknownDiceSectionError
	var unknownRow *template.UnknownDiceRowError
	var invalid *template.InvalidDiceError
	var tooMany *template.TooManyDiceOverridesError
	if errors.As(err, &unknownSection) || errors.As(err, &unknownRow) || errors.As(err, &invalid) || errors.As(err, &tooMany) {
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid dice: "+err.Error(), err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to validate dice", err)
}

// translateRepoErr maps repository errors to typed apperr at the service
// boundary. The repository layer must stay free of apperr imports.
func translateRepoErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, ErrSheetNotFound) {
		return apperr.NewError(apperr.CodeNotFound, "sheet not found", err)
	}
	if errors.Is(err, ErrSheetOwnerExists) {
		return apperr.NewError(apperr.CodeSheetLimitReached, MsgSheetLimitReached, err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to "+op+" sheet", err)
}

// translateDAGErr maps template DAG errors to typed apperr at the service
// boundary. The template/DAG layer must stay free of apperr imports.
func translateDAGErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	var unknownField *template.UnknownFieldError
	if errors.As(err, &unknownField) {
		return apperr.NewError(apperr.CodeUnknownField, MsgFieldDoesNotExist+unknownField.Key, nil)
	}
	var notEditable *template.NotEditableError
	if errors.As(err, &notEditable) {
		return apperr.NewError(apperr.CodeInvalidRequest, notEditable.Error(), nil)
	}
	var conflict *template.ExclusiveGroupConflictError
	if errors.As(err, &conflict) {
		return apperr.NewError(
			apperr.CodeInvalidRequest,
			"conflicting exclusive options: "+conflict.Group,
			err,
		)
	}
	var invalidExclusive *template.ExclusiveValueTypeError
	if errors.As(err, &invalidExclusive) {
		return apperr.NewError(
			apperr.CodeInvalidRequest,
			"exclusive option must be numeric: "+invalidExclusive.Key,
			err,
		)
	}
	if errors.Is(err, maputils.ErrMaxDepthExceeded) {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgNestedTooDeep, err)
	}
	switch {
	case isMaputilsDataErr(err):
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid sheet data", err)
	case errors.Is(err, template.ErrFormulaEval):
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	case errors.Is(err, template.ErrNilOriginalData), errors.Is(err, template.ErrNilUpdatedData):
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	case errors.Is(err, template.ErrInfiniteCycle):
		return apperr.NewError(apperr.CodeCycleDetected, err.Error(), nil)
	}
	return apperr.NewError(apperr.CodeInternal, "dag recalculation failed", err)
}

// translateMapErr maps backend/shared/maputils errors to typed apperr at the service
// boundary. The maputils layer must stay free of apperr imports.
func translateMapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, maputils.ErrMaxDepthExceeded) {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgNestedTooDeep, err)
	}
	if isMaputilsDataErr(err) {
		return apperr.NewError(apperr.CodeInvalidRequest, "invalid sheet data", err)
	}
	return apperr.NewError(apperr.CodeInternal, "failed to "+op+" sheet", err)
}

// isMaputilsDataErr reports whether err is a typed maputils nested-data error
// (empty path, nil map or non-map intermediate value).
func isMaputilsDataErr(err error) bool {
	return errors.Is(err, maputils.ErrEmptyPath) ||
		errors.Is(err, maputils.ErrNilMap) ||
		errors.Is(err, maputils.ErrNotMap)
}

// nestedCascadingEffects converts the flat path-key cascading effects
// returned by DAG.Recalculate back into a nested map so the API response
// mirrors the nested request payload.
func nestedCascadingEffects(flat map[string]any) (map[string]any, error) {
	nested := make(map[string]any)
	if err := maputils.ApplyUpdates(nested, flat); err != nil {
		return nil, translateMapErr("update", err)
	}
	return nested, nil
}

// buildNestedAllFields creates nested map with ALL leaf fields (base + derived)
// seeded through FieldSchema.SeedValue: numbers start at their default, text
// starts empty; derived leaves are overwritten by the Recalculate call that
// follows.
func buildNestedAllFields(group template.FieldGroup) map[string]any {
	result := make(map[string]any)
	for name, node := range group {
		if node.IsLeaf() {
			result[name] = node.Schema.SeedValue()
		} else if node.IsGroup() {
			nested := buildNestedAllFields(node.Children)
			if len(nested) > 0 {
				result[name] = nested
			}
		}
	}
	return result
}
