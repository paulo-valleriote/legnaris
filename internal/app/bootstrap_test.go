package app_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	"legnaris/internal/engine"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// buildModules builds the production module graph over a mongo client that
// never reaches a server, so the boot-time configuration checks are observable
// with no database. secretPresent controls whether EMAIL_CODE_SECRET is set at
// all, so the absent case is distinct from the blank one.
func buildModules(t *testing.T, secret string, secretPresent bool) (*app.Modules, error) {
	t.Helper()

	t.Setenv("JWT_SECRET", "bootstrap-test-secret")
	t.Setenv("AUTH_BCRYPT_COST", "12")
	if secretPresent {
		t.Setenv("EMAIL_CODE_SECRET", secret)
	} else {
		old, had := os.LookupEnv("EMAIL_CODE_SECRET")
		require.NoError(t, os.Unsetenv("EMAIL_CODE_SECRET"))
		t.Cleanup(func() {
			if had {
				_ = os.Setenv("EMAIL_CODE_SECRET", old)
			} else {
				_ = os.Unsetenv("EMAIL_CODE_SECRET")
			}
		})
	}

	log := logger.NewLogger(slog.LevelError)
	cfg := env.NewEnv(log)

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=50"))
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	})

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	return app.NewModules(&db.Database{Client: client}, registry, cfg, auth.NewNopObserver(), auth.NewNopChallengeObserver())
}

// TestNewModules_EmailCodeSecret pins the one required setting this feature
// adds: an absent, blank or too-short digest secret fails the boot with the
// typed sentinel, while a valid one starts (OTP-18, SC-008).
func TestNewModules_EmailCodeSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		present bool
	}{
		{name: "Edge: an absent secret fails the boot", secret: "", present: false},
		{name: "Edge: a blank secret fails the boot", secret: "", present: true},
		{name: "Edge: a whitespace-only secret fails the boot", secret: strings.Repeat(" ", 40), present: true},
		{name: "Edge: a 31-byte secret fails the boot", secret: strings.Repeat("k", 31), present: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modules, err := buildModules(t, tc.secret, tc.present)

			assert.Nil(t, modules, "a weak secret must not produce a half-wired server")
			assert.ErrorIs(t, err, auth.ErrCodeSecretWeak)
		})
	}

	t.Run("Happy path: a 32-byte secret starts the server", func(t *testing.T) {
		modules, err := buildModules(t, strings.Repeat("k", 32), true)

		require.NoError(t, err)
		require.NotNil(t, modules)
		assert.NotNil(t, modules.Auth.Service, "the challenge service must be wired")
	})
}
