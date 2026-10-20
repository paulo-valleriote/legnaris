package app

import (
	"net/http"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/json"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

type SheetController struct {
	service sheet.Service
}

func NewSheetController(service sheet.Service) *SheetController {
	return &SheetController{
		service: service,
	}
}

func (sc *SheetController) RegisterRoutes(group *echo.Group) {
	group.POST("", sc.Create)
	group.GET("", sc.FetchMySheet)
	group.GET("/:id", sc.FetchByID)
	group.PATCH("/:id", sc.PartialUpdate)
	group.DELETE("/:id", sc.Delete)
	group.POST("/:id/preview", sc.Preview)
}

// RegisterTemplatePreviewRoute exposes the template dry-run on the templates
// group. The operation lives on the sheet service because it shares the
// recalculation path, but it is grouped with the other /templates routes for
// the client.
func (sc *SheetController) RegisterTemplatePreviewRoute(group *echo.Group) {
	group.POST("/:id/preview", sc.PreviewTemplate)
}

func (sc *SheetController) Create(c *echo.Context) error {
	var cmd sheet.CreateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	cmd.OwnerID, _ = auth.UserID(c)

	newSheet, err := sc.service.Create(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, newSheet.ToDTO())
}

func (sc *SheetController) FetchMySheet(c *echo.Context) error {
	ownerID, _ := auth.UserID(c)
	storedSheet, err := sc.service.FetchMySheet(c.Request().Context(), ownerID)
	if err != nil {
		return err
	}
	return response.OK(c, storedSheet.ToDTO())
}

func (sc *SheetController) FetchByID(c *echo.Context) error {
	ownerID, _ := auth.UserID(c)
	storedSheet, err := sc.service.FetchByID(c.Request().Context(), c.Param("id"), ownerID)
	if err != nil {
		return err
	}
	return response.OK(c, storedSheet.ToDTO())
}

func (sc *SheetController) PartialUpdate(c *echo.Context) error {
	var cmd sheet.PartialUpdateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	ownerID, _ := auth.UserID(c)
	cascadingEffects, err := sc.service.PartialUpdate(c.Request().Context(), sheet.PartialUpdateSheetCommand{
		SheetID: c.Param("id"),
		OwnerID: ownerID,
		Data:    cmd.Data,
	})
	if err != nil {
		return err
	}
	return response.OK(c, cascadingEffects)
}

// Preview is the stateless dry-run of PartialUpdate: same body, same cascade,
// same typed errors, no persistence. It exists so a client can show computed
// values before committing.
func (sc *SheetController) Preview(c *echo.Context) error {
	var cmd sheet.PartialUpdateSheetCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	ownerID, _ := auth.UserID(c)
	cascadingEffects, err := sc.service.Preview(c.Request().Context(), sheet.PartialUpdateSheetCommand{
		SheetID: c.Param("id"),
		OwnerID: ownerID,
		Data:    cmd.Data,
	})
	if err != nil {
		return err
	}
	return response.OK(c, cascadingEffects)
}

// Delete removes the authenticated owner's sheet and returns 204. A foreign or
// already-deleted sheet surfaces as 404 from the service, so a repeat delete is
// not a silent success.
func (sc *SheetController) Delete(c *echo.Context) error {
	ownerID, _ := auth.UserID(c)

	if err := sc.service.Delete(c.Request().Context(), sheet.DeleteSheetCommand{
		SheetID: c.Param("id"),
		OwnerID: ownerID,
	}); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// PreviewTemplate is the template-seeded dry-run: it decodes the data delta,
// takes the template id from the path and returns the cascade with no stored
// sheet and no persistence.
func (sc *SheetController) PreviewTemplate(c *echo.Context) error {
	var cmd sheet.PreviewTemplateCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	cascadingEffects, err := sc.service.PreviewTemplate(c.Request().Context(), sheet.PreviewTemplateCommand{
		TemplateID: c.Param("id"),
		Data:       cmd.Data,
	})
	if err != nil {
		return err
	}
	return response.OK(c, cascadingEffects)
}
