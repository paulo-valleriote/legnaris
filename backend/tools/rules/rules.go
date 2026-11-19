//go:build ruleguard

package rules

import "github.com/quasilyte/go-ruleguard/dsl"

func apperrBareCause(m dsl.Matcher) {
	m.Match(`apperr.NewError($_, $_, errors.New($_))`).
		Where(!m.File().Name.Matches(`_test\.go$`)).
		Report(`cause must be nil or a lower-layer typed error; return the typed error from the lower layer and translate it with apperr.NewError at the boundary`)
}

func apperrUnwrappedCause(m dsl.Matcher) {
	m.Match(`apperr.NewError($_, $_, fmt.Errorf($format, $*_))`).
		Where(!m.File().Name.Matches(`_test\.go$`) &&
			m["format"].Text.Matches(`^"`) &&
			!m["format"].Text.Matches(`%w`)).
		Report(`wrap the cause with %w so errors.Is/errors.As still reach the lower-layer typed error`)
}

func serviceErrorSentinel(m dsl.Matcher) {
	m.Match(`var $name = errors.New($_)`).
		Where(m["name"].Object.IsGlobal() &&
			m.File().PkgPath.Matches(`^legnaris/backend/internal/(app|sheet)$`) &&
			!m.File().Name.Matches(`_test\.go$`)).
		Report(`no package-level errors.New sentinels in backend/internal/app or backend/internal/sheet; return apperr.NewError with a nil or lower-layer cause instead`)
}

func emptyPrefixGroupMiddleware(m dsl.Matcher) {
	m.Match(`$router.Group("", $middleware, $*_)`).
		Report(`attach middleware with Use on the route, an empty-prefix group turns 405 into another status`)
}
