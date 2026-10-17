package auth

import "net/http"

// RefreshCookieName and CSRFCookieName are contract-fixed: the __Host- prefix,
// the names and the attribute set are part of the client contract, never
// configuration (AUTH-01, AUTH-09, AUTH-24).
const (
	RefreshCookieName = "__Host-legnaris_refresh"
	CSRFCookieName    = "__Host-legnaris_csrf"
)

// cookiePath is the only path __Host- cookies may use.
const cookiePath = "/"

// RefreshCookie builds the HttpOnly cookie that carries the raw refresh
// credential. maxAge is the refresh lifetime in whole seconds; the value never
// appears in a response body (AUTH-01, AUTH-03, AUTH-08).
func RefreshCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     RefreshCookieName,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// CSRFCookie builds the script-readable CSRF cookie the page echoes back in the
// X-CSRF-Token header. It is deliberately not HttpOnly (AUTH-09).
func CSRFCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CSRFCookieName,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// ClearRefreshCookie expires the refresh cookie on logout: the same name and
// attributes with an empty value and MaxAge<0 (rendered Max-Age=0), so the
// browser drops exactly the cookie that was set (AUTH-04).
func ClearRefreshCookie() *http.Cookie {
	return RefreshCookie("", -1)
}

// ClearCSRFCookie expires the CSRF cookie on logout alongside the refresh
// cookie (AUTH-12).
func ClearCSRFCookie() *http.Cookie {
	return CSRFCookie("", -1)
}
