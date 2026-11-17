package campaigntest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/shared/idgen"
)

var fixedNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

func Now() time.Time {
	return fixedNow
}

func NewID(t testing.TB) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}

func Build(t testing.TB, gms, players int) campaign.Campaign {
	t.Helper()
	owner := NewID(t)
	c := campaign.Campaign{
		ID:         NewID(t),
		Name:       "Curse of Strahd",
		TemplateID: "dnd5e",
		Status:     campaign.StatusActive,
		Version:    1,
		Members: []campaign.Member{
			{UserID: owner, Role: campaign.RoleGM, Owner: true, JoinedAt: fixedNow},
		},
		CreatedAt: fixedNow,
		UpdatedAt: fixedNow,
	}
	for range gms {
		c.Members = append(c.Members, campaign.Member{UserID: NewID(t), Role: campaign.RoleGM, JoinedAt: fixedNow})
	}
	for range players {
		c.Members = append(c.Members, campaign.Member{UserID: NewID(t), Role: campaign.RolePlayer, JoinedAt: fixedNow})
	}
	c.GMCount = 1 + gms
	c.PlayerCount = players
	return c
}

func Copy(c campaign.Campaign) campaign.Campaign {
	out := c
	out.Members = append([]campaign.Member(nil), c.Members...)
	return out
}

func OwnerID(c campaign.Campaign) string {
	for _, m := range c.Members {
		if m.Owner {
			return m.UserID
		}
	}
	return ""
}

func MembersWithRole(c campaign.Campaign, role campaign.Role) []string {
	var ids []string
	for _, m := range c.Members {
		if m.Role == role && !m.Owner {
			ids = append(ids, m.UserID)
		}
	}
	return ids
}
