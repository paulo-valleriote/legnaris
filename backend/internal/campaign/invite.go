package campaign

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"unicode/utf8"

	"legnaris/backend/shared/apperr"
)

const (
	inviteCodeBytes = 18
	passwordMinRune = 8
	passwordMaxRune = 128
)

var inviteCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{24}$`)

func newInviteCode() (string, error) {
	buf := make([]byte, inviteCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validInviteCode(code string) bool {
	return inviteCodePattern.MatchString(code)
}

func ValidPassword(password string) bool {
	n := utf8.RuneCountInString(password)
	return n >= passwordMinRune && n <= passwordMaxRune
}

func invalidInvite() *apperr.Error {
	return apperr.NewError(apperr.CodeInvalidInvite, MsgInviteInvalid, nil)
}
