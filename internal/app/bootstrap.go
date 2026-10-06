package app

import (
	"fmt"

	"legnaris/internal/auth"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/internal/user"
	"legnaris/shared/db"
	"legnaris/shared/env"
)

type TemplateModule struct {
	Controller *TemplateController
	Service    template.Service
}

type SheetModule struct {
	Controller *SheetController
	Service    sheet.Service
}

type AuthModule struct {
	Controller *AuthController
	Service    auth.Service
}

type Modules struct {
	Template TemplateModule
	Sheet    SheetModule
	Auth     AuthModule
}

// NewModules wires every domain module. It returns an error for configuration
// that must fail fast at boot (JWT secret, bcrypt cost) instead of producing a
// half-usable server.
func NewModules(database *db.Database, registry *template.Registry, cfg env.Env, observer auth.Observer) (*Modules, error) {
	log := cfg.Logger
	mdb := database.Client.Database(cfg.Variables.DatabaseName)

	templateService := template.NewService(registry)
	templateController := NewTemplateController(templateService)

	sheetRepository := sheet.NewRepository(mdb, log)
	sheetService := sheet.NewService(sheetRepository, templateService, registry)
	sheetController := NewSheetController(sheetService)

	userRepository := user.NewRepository(mdb, log)
	refreshTokenRepository := auth.NewRefreshTokenRepository(mdb)

	hasher, err := auth.NewPasswordHasher(cfg.Variables.AuthBcryptCost)
	if err != nil {
		return nil, fmt.Errorf("build password hasher: %w", err)
	}

	tokenManager, err := auth.NewTokenManager(cfg.Variables.JWTSecret, cfg.Variables.JWTAccessTTL)
	if err != nil {
		return nil, fmt.Errorf("build token manager: %w", err)
	}

	authService := auth.NewService(
		userRepository,
		refreshTokenRepository,
		hasher,
		tokenManager,
		auth.NewRefreshTokenGenerator(),
		observer,
		auth.Config{
			AccessTTL:  cfg.Variables.JWTAccessTTL,
			RefreshTTL: cfg.Variables.AuthRefreshTTL,
		},
		log,
	)
	authController := NewAuthController(authService)

	return &Modules{
		Template: TemplateModule{
			Controller: templateController,
			Service:    templateService,
		},
		Sheet: SheetModule{
			Controller: sheetController,
			Service:    sheetService,
		},
		Auth: AuthModule{
			Controller: authController,
			Service:    authService,
		},
	}, nil
}
