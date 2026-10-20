package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"legnaris/backend/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cookie contract is fixed: the names carry the __Host- prefix and the
// attribute set can never be relaxed, so every field and the rendered header
// are both pinned (AUTH-01, AUTH-09, AUTH-24).
func TestRefreshCookie(t *testing.T) {
	t.Run("Happy path: the set builder carries the full attribute set", func(t *testing.T) {
		cookie := auth.RefreshCookie("raw-refresh-credential", 720)

		require.NotNil(t, cookie)
		assert.Equal(t, auth.RefreshCookieName, cookie.Name)
		assert.True(t, strings.HasPrefix(cookie.Name, "__Host-"), "the name must carry the __Host- prefix")
		assert.Equal(t, "raw-refresh-credential", cookie.Value)
		assert.True(t, cookie.HttpOnly, "the refresh credential must be unreadable by script")
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
		assert.Equal(t, "/", cookie.Path)
		assert.Empty(t, cookie.Domain, "__Host- forbids a Domain attribute")
		assert.Equal(t, 720, cookie.MaxAge)

		rendered := cookie.String()
		assert.Contains(t, rendered, auth.RefreshCookieName+"=")
		assert.Contains(t, rendered, "Path=/")
		assert.Contains(t, rendered, "Max-Age=720")
		assert.Contains(t, rendered, "HttpOnly")
		assert.Contains(t, rendered, "Secure")
		assert.Contains(t, rendered, "SameSite=Strict")
		assert.NotContains(t, rendered, "Domain", "__Host- forbids a Domain attribute")
	})

	t.Run("Edge: a negative lifetime renders as Max-Age=0 (expired on write)", func(t *testing.T) {
		cookie := auth.RefreshCookie("value", -1)

		assert.Equal(t, -1, cookie.MaxAge)
		assert.Contains(t, cookie.String(), "Max-Age=0")
	})
}

func TestCSRFCookie(t *testing.T) {
	t.Run("Happy path: the CSRF cookie is script-readable and secure", func(t *testing.T) {
		cookie := auth.CSRFCookie("csrf-token", 720)

		require.NotNil(t, cookie)
		assert.Equal(t, auth.CSRFCookieName, cookie.Name)
		assert.True(t, strings.HasPrefix(cookie.Name, "__Host-"), "the name must carry the __Host- prefix")
		assert.Equal(t, "csrf-token", cookie.Value)
		assert.False(t, cookie.HttpOnly, "the page must be able to read and echo the CSRF token")
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
		assert.Equal(t, "/", cookie.Path)
		assert.Empty(t, cookie.Domain)
		assert.Equal(t, 720, cookie.MaxAge)

		rendered := cookie.String()
		assert.Contains(t, rendered, auth.CSRFCookieName+"=")
		assert.Contains(t, rendered, "Path=/")
		assert.Contains(t, rendered, "Max-Age=720")
		assert.NotContains(t, rendered, "HttpOnly")
		assert.Contains(t, rendered, "Secure")
		assert.Contains(t, rendered, "SameSite=Strict")
	})
}

func TestClearCookies(t *testing.T) {
	t.Run("Happy path: clearing the refresh cookie keeps the name and expires it", func(t *testing.T) {
		cookie := auth.ClearRefreshCookie()

		require.NotNil(t, cookie)
		assert.Equal(t, auth.RefreshCookieName, cookie.Name)
		assert.Empty(t, cookie.Value)
		assert.Equal(t, -1, cookie.MaxAge)
		assert.True(t, cookie.HttpOnly)
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
		assert.Equal(t, "/", cookie.Path)
		assert.Empty(t, cookie.Domain)

		rendered := cookie.String()
		assert.Contains(t, rendered, auth.RefreshCookieName+"=;")
		assert.Contains(t, rendered, "Max-Age=0")
		assert.Contains(t, rendered, "Path=/")
		assert.Contains(t, rendered, "HttpOnly")
		assert.Contains(t, rendered, "Secure")
		assert.Contains(t, rendered, "SameSite=Strict")
	})

	t.Run("Happy path: clearing the CSRF cookie keeps the name and expires it", func(t *testing.T) {
		cookie := auth.ClearCSRFCookie()

		require.NotNil(t, cookie)
		assert.Equal(t, auth.CSRFCookieName, cookie.Name)
		assert.Empty(t, cookie.Value)
		assert.Equal(t, -1, cookie.MaxAge)
		assert.False(t, cookie.HttpOnly)
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
		assert.Equal(t, "/", cookie.Path)
		assert.Empty(t, cookie.Domain)

		rendered := cookie.String()
		assert.Contains(t, rendered, auth.CSRFCookieName+"=;")
		assert.Contains(t, rendered, "Max-Age=0")
		assert.NotContains(t, rendered, "HttpOnly")
	})

	t.Run("Edge: the clear and set builders share the exact name and attributes", func(t *testing.T) {
		set := auth.RefreshCookie("value", 720)
		clear := auth.ClearRefreshCookie()

		assert.Equal(t, set.Name, clear.Name)
		assert.Equal(t, set.Path, clear.Path)
		assert.Equal(t, set.Domain, clear.Domain)
		assert.Equal(t, set.HttpOnly, clear.HttpOnly)
		assert.Equal(t, set.Secure, clear.Secure)
		assert.Equal(t, set.SameSite, clear.SameSite)

		csrfSet := auth.CSRFCookie("value", 720)
		csrfClear := auth.ClearCSRFCookie()
		assert.Equal(t, csrfSet.Name, csrfClear.Name)
		assert.Equal(t, csrfSet.HttpOnly, csrfClear.HttpOnly)
		assert.Equal(t, csrfSet.Path, csrfClear.Path)
	})
}
