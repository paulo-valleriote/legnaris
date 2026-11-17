package campaign

import (
	"regexp"
	"time"

	"legnaris/backend/shared/apperr"
)

type Role string

const (
	RoleGM     Role = "gm"
	RolePlayer Role = "player"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDeleting Status = "deleting"
)

const (
	MaxPlayers = 10
	MaxGMs     = 3
)

const (
	nameMaxRunes = 80
	nameMinRunes = 1
)

var templateIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Member struct {
	UserID   string    `bson:"user_id"`
	Role     Role      `bson:"role"`
	Owner    bool      `bson:"owner,omitempty"`
	JoinedAt time.Time `bson:"joined_at"`
}

type Campaign struct {
	ID           string    `bson:"_id"`
	Name         string    `bson:"name"`
	TemplateID   string    `bson:"template_id"`
	Status       Status    `bson:"status"`
	InviteCode   string    `bson:"invite_code,omitempty"`
	PasswordHash string    `bson:"password_hash,omitempty"`
	Version      int64     `bson:"version"`
	GMCount      int       `bson:"gm_count"`
	PlayerCount  int       `bson:"player_count"`
	Members      []Member  `bson:"members"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func (c *Campaign) Active() bool {
	return c.Status == StatusActive
}

func (c *Campaign) MemberOf(userID string) (Member, bool) {
	if userID == "" {
		return Member{}, false
	}
	for _, m := range c.Members {
		if m.UserID == userID {
			return m, true
		}
	}
	return Member{}, false
}

func (c *Campaign) indexOf(userID string) int {
	for i, m := range c.Members {
		if m.UserID == userID {
			return i
		}
	}
	return -1
}

func (c *Campaign) recount() {
	c.GMCount = 0
	c.PlayerCount = 0
	for _, m := range c.Members {
		if m.Role == RoleGM {
			c.GMCount++
		} else {
			c.PlayerCount++
		}
	}
}

func ValidName(name string) bool {
	n := len([]rune(name))
	return n >= nameMinRunes && n <= nameMaxRunes
}

func ValidTemplateID(id string) bool {
	return templateIDPattern.MatchString(id)
}

func notFound() *apperr.Error {
	return apperr.NewError(apperr.CodeCampaignNotFound, MsgCampaignNotFound, nil)
}
