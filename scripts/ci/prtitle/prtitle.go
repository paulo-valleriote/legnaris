package prtitle

import (
	"errors"
	"fmt"
	"regexp"
)

var ErrMissingKey = errors.New("pull request title has no LGN-<number> Jira key")

var jiraKey = regexp.MustCompile(`LGN-[0-9]+`)

func Validate(title string) error {
	if !jiraKey.MatchString(title) {
		return fmt.Errorf("%w: %q", ErrMissingKey, title)
	}
	return nil
}
