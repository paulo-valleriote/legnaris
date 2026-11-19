package campaign

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

type CharacterStore interface {
	FetchByID(ctx context.Context, id string) (*sheet.Sheet, error)
	LinkCampaign(ctx context.Context, id string, expectedCampaignID *string, campaignID string) (bool, error)
	UnlinkOwnerCampaign(ctx context.Context, ownerID, campaignID string) (int64, error)
	ListByCampaignID(ctx context.Context, campaignID string) ([]sheet.Sheet, error)
}

type LinkCommand struct {
	CampaignID string `json:"-"`
	UserID     string `json:"-"`
	SheetID    string `json:"sheet_id"`
}

type UnlinkCommand struct {
	CampaignID string `json:"-"`
	UserID     string `json:"-"`
}

type LinkedSheetSummary struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	TemplateID string    `json:"template_id"`
	OwnerID    string    `json:"owner_id"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ReadLinkedSheetQuery struct {
	CampaignID  string
	ViewerID    string
	SheetID     string
	IfNoneMatch string
}

type LinkedSheetResult struct {
	ETag        string
	NotModified bool
	Sheet       *sheet.Sheet
}

const (
	MsgSheetNotFound      = "sheet not found"
	MsgSheetIDInvalid     = "sheet id must be a canonical UUID v7"
	MsgSheetIDMissing     = "sheet id not provided"
	MsgNotACharacter      = "only character sheets can be linked to a campaign"
	MsgTemplateMismatch   = "sheet template does not match the campaign template"
	MsgSheetAlreadyLinked = "sheet is already linked to a campaign"
	MsgGMHasNoCharacter   = "game masters cannot link a character"
	MsgListSheetsGMOnly   = "only game masters can list linked sheets"
	MsgCharacterRead      = "failed to read linked characters"
	MsgCharacterWrite     = "failed to update character link"
	MsgSheetRead          = "failed to read sheet"
)

type noCharacters struct{}

func (noCharacters) FetchByID(context.Context, string) (*sheet.Sheet, error) {
	return nil, sheeterr.ErrSheetNotFound
}

func (noCharacters) LinkCampaign(context.Context, string, *string, string) (bool, error) {
	return false, nil
}

func (noCharacters) UnlinkOwnerCampaign(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (noCharacters) ListByCampaignID(context.Context, string) ([]sheet.Sheet, error) {
	return nil, nil
}

func requireSheetID(id string) error {
	if id == "" {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDMissing, nil)
	}
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgSheetIDInvalid, err)
	}
	return nil
}

func sheetNotFound() *apperr.Error {
	return apperr.NewError(apperr.CodeNotFound, MsgSheetNotFound, nil)
}

func alreadyLinked() *apperr.Error {
	return apperr.NewError(apperr.CodeSheetAlreadyLinked, MsgSheetAlreadyLinked, nil)
}

func (s *ServiceImpl) Link(ctx context.Context, cmd LinkCommand) (View, error) {
	if err := requireUser(cmd.UserID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return View{}, err
	}
	var replaced int64
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		member, ok := c.MemberOf(cmd.UserID)
		if !ok {
			return notFound()
		}
		if member.Role != RolePlayer {
			return forbidden(MsgGMHasNoCharacter)
		}
		n, err := s.claimSheet(txCtx, c, cmd.UserID, cmd.SheetID)
		replaced = n
		return err
	})
	if err != nil {
		return View{}, err
	}
	s.publishCampaignEvent(EventSheetLinked, cmd.UserID, out.ID, out.Version, map[string]any{
		"sheet_id": cmd.SheetID,
	})
	if replaced > 0 {
		s.publishCampaignEvent(EventSheetUnlinked, cmd.UserID, out.ID, out.Version, map[string]any{
			dataKeyUserID: cmd.UserID,
		})
	}
	return s.view(ctx, &out, cmd.UserID), nil
}

func (s *ServiceImpl) claimSheet(txCtx context.Context, c *Campaign, userID, sheetID string) (int64, error) {
	stored, err := s.sheets.FetchByID(txCtx, sheetID)
	if errors.Is(err, sheeterr.ErrSheetNotFound) {
		return 0, sheetNotFound()
	}
	if err != nil {
		return 0, internalError(MsgSheetRead, err)
	}
	if stored.OwnerID != userID {
		return 0, sheetNotFound()
	}
	if stored.Kind != plan.KindCharacter {
		return 0, apperr.NewError(apperr.CodeInvalidRequest, MsgNotACharacter, nil)
	}
	if stored.TemplateID != c.TemplateID {
		return 0, apperr.NewError(apperr.CodeTemplateMismatch, MsgTemplateMismatch, nil)
	}
	if err := s.ensureUnclaimed(txCtx, stored); err != nil {
		return 0, err
	}
	replaced, err := s.sheets.UnlinkOwnerCampaign(txCtx, userID, c.ID)
	if err != nil {
		return 0, internalError(MsgCharacterWrite, err)
	}
	linked, err := s.sheets.LinkCampaign(txCtx, stored.ID, stored.CampaignID, c.ID)
	if err != nil {
		return 0, internalError(MsgCharacterWrite, err)
	}
	if !linked {
		return 0, alreadyLinked()
	}
	return replaced, nil
}

func (s *ServiceImpl) ensureUnclaimed(ctx context.Context, stored *sheet.Sheet) error {
	if stored.CampaignID == nil {
		return nil
	}
	other, err := s.repository.FetchByID(ctx, *stored.CampaignID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return internalError(MsgCampaignRead, err)
	}
	if !other.Active() {
		return nil
	}
	return alreadyLinked()
}

func (s *ServiceImpl) Unlink(ctx context.Context, cmd UnlinkCommand) (View, error) {
	if err := requireUser(cmd.UserID); err != nil {
		return View{}, err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return View{}, err
	}
	var removed int64
	out, err := Mutate(ctx, s.tx, s.repository, cmd.CampaignID, s.now(), func(txCtx context.Context, c *Campaign) error {
		member, ok := c.MemberOf(cmd.UserID)
		if !ok {
			return notFound()
		}
		n, err := s.sheets.UnlinkOwnerCampaign(txCtx, member.UserID, c.ID)
		if err != nil {
			return internalError(MsgCharacterWrite, err)
		}
		removed = n
		return nil
	})
	if err != nil {
		return View{}, err
	}
	if removed > 0 {
		s.publishCampaignEvent(EventSheetUnlinked, cmd.UserID, out.ID, out.Version, map[string]any{
			dataKeyUserID: cmd.UserID,
		})
	}
	return s.view(ctx, &out, cmd.UserID), nil
}

func (s *ServiceImpl) ListLinkedSheets(ctx context.Context, campaignID, viewerID string) ([]LinkedSheetSummary, error) {
	if err := requireUser(viewerID); err != nil {
		return nil, err
	}
	if err := requireCampaignID(campaignID); err != nil {
		return nil, err
	}
	c, err := s.activeCampaignFor(ctx, campaignID, viewerID)
	if err != nil {
		return nil, err
	}
	member, _ := c.MemberOf(viewerID)
	if member.Role != RoleGM {
		return nil, forbidden(MsgListSheetsGMOnly)
	}
	found, err := s.sheets.ListByCampaignID(ctx, campaignID)
	if err != nil {
		return nil, internalError(MsgCharacterRead, err)
	}
	summaries := make([]LinkedSheetSummary, 0, len(found))
	for i := range found {
		summaries = append(summaries, LinkedSheetSummary{
			ID:         found[i].ID,
			Name:       found[i].Name,
			TemplateID: found[i].TemplateID,
			OwnerID:    found[i].OwnerID,
			UpdatedAt:  found[i].UpdatedAt,
		})
	}
	return summaries, nil
}

func (s *ServiceImpl) ReadLinkedSheet(ctx context.Context, q ReadLinkedSheetQuery) (LinkedSheetResult, error) {
	if err := requireUser(q.ViewerID); err != nil {
		return LinkedSheetResult{}, err
	}
	if err := requireCampaignID(q.CampaignID); err != nil {
		return LinkedSheetResult{}, err
	}
	if err := requireSheetID(q.SheetID); err != nil {
		return LinkedSheetResult{}, err
	}
	c, err := s.activeCampaignFor(ctx, q.CampaignID, q.ViewerID)
	if err != nil {
		return LinkedSheetResult{}, err
	}
	member, _ := c.MemberOf(q.ViewerID)
	stored, err := s.sheets.FetchByID(ctx, q.SheetID)
	if errors.Is(err, sheeterr.ErrSheetNotFound) {
		return LinkedSheetResult{}, sheetNotFound()
	}
	if err != nil {
		return LinkedSheetResult{}, internalError(MsgSheetRead, err)
	}
	if member.Role != RoleGM && stored.OwnerID != member.UserID {
		return LinkedSheetResult{}, forbidden(MsgForbidden)
	}
	if stored.CampaignID == nil || *stored.CampaignID != c.ID {
		return LinkedSheetResult{}, sheetNotFound()
	}
	etag := SheetETag(stored.Version, stored.LayoutVersion, member.Role)
	if ETagMatches(q.IfNoneMatch, etag) {
		return LinkedSheetResult{ETag: etag, NotModified: true}, nil
	}
	return LinkedSheetResult{ETag: etag, Sheet: stored}, nil
}

func (s *ServiceImpl) activeCampaignFor(ctx context.Context, campaignID, viewerID string) (*Campaign, error) {
	c, err := s.repository.FetchByID(ctx, campaignID)
	if err != nil {
		return nil, readError(err)
	}
	if _, ok := c.MemberOf(viewerID); !ok || !c.Active() {
		return nil, notFound()
	}
	return c, nil
}

func (s *ServiceImpl) characterNames(ctx context.Context, campaignID string) map[string]string {
	found, err := s.sheets.ListByCampaignID(ctx, campaignID)
	if err != nil {
		return nil
	}
	names := make(map[string]string, len(found))
	for i := range found {
		names[found[i].OwnerID] = found[i].Name
	}
	return names
}
