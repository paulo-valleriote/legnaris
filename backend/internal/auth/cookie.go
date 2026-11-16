package auth

import "net/http"

const (
	RefreshCookieName = "__Host-legnaris_refresh"
	CSRFCookieName    = "__Host-legnaris_csrf"
)

const cookiePath = "/"

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

func ClearRefreshCookie() *http.Cookie {
	return RefreshCookie("", -1)
}

func ClearCSRFCookie() *http.Cookie {
	return CSRFCookie("", -1)
}
