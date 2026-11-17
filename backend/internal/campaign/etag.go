package campaign

import (
	"strconv"
	"strings"
)

func SheetETag(sheetVersion, layoutVersion int64, role Role) string {
	return `"` + strconv.FormatInt(sheetVersion, 10) + "." + strconv.FormatInt(layoutVersion, 10) + "." + string(role) + `"`
}

func ETagMatches(ifNoneMatch, etag string) bool {
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		if strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}
