package app

import (
	"net/http"

	"legnaris/internal/auth"
	"legnaris/shared/json"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
)

// RefreshTokenHeader carries the opaque refresh token on /auth/refresh and
// /auth/logout. Native clients (desktop/mobile) store it in the OS secure
// storage and resend it here.
const RefreshTokenHeader = "X-Refresh-Token"

type AuthController struct {
	service auth.Service
}

func NewAuthController(service auth.Service) *AuthController {
	return &AuthController{
		service: service,
	}
}

func (ac *AuthController) RegisterRoutes(group *echo.Group) {
	group.POST("/register", ac.Register)
	group.POST("/login", ac.Login)
	group.POST("/refresh", ac.Refresh)
	group.POST("/logout", ac.Logout)
}

func (ac *AuthController) Register(c *echo.Context) error {
	var cmd auth.RegisterCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	created, err := ac.service.Register(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.Created(c, created.ToDTO())
}

func (ac *AuthController) Login(c *echo.Context) error {
	var cmd auth.LoginCommand

	if err := json.Decode(c, &cmd); err != nil {
		return decodeError(err)
	}

	pair, err := ac.service.Login(c.Request().Context(), cmd)
	if err != nil {
		return err
	}
	return response.OK(c, pair)
}

// Refresh and Logout take the refresh token from the X-Refresh-Token header and
// have no request body.
func (ac *AuthController) Refresh(c *echo.Context) error {
	pair, err := ac.service.Refresh(c.Request().Context(), auth.RefreshCommand{
		RefreshToken: c.Request().Header.Get(RefreshTokenHeader),
	})
	if err != nil {
		return err
	}
	return response.OK(c, pair)
}

func (ac *AuthController) Logout(c *echo.Context) error {
	err := ac.service.Logout(c.Request().Context(), auth.LogoutCommand{
		RefreshToken: c.Request().Header.Get(RefreshTokenHeader),
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
