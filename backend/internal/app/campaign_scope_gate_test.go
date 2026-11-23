package app

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/campaign"
	campaignmocks "legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const gateCampaignID = "01900000-0000-7000-8000-0000000000cc"
const gateUserID = "01900000-0000-7000-8000-0000000000aa"
const gateTemplateID = "retired_v1"

func TestCampaignScopeGate_AllowsTemplate(t *testing.T) {
	internalErr := errors.New("campaign store unavailable")
	tests := []struct {
		name     string
		view     campaign.View
		fetchErr error
		want     bool
		wantErr  bool
	}{
		{
			name: "Happy path: an active member campaign whose scope holds the template allows it",
			view: campaign.View{Status: campaign.StatusActive, TemplateIDs: []string{gateTemplateID}},
			want: true,
		},
		{
			name: "Edge: a template outside the scope is refused",
			view: campaign.View{Status: campaign.StatusActive, TemplateIDs: []string{"other_v1"}},
			want: false,
		},
		{
			name: "Edge: an empty scope is refused",
			view: campaign.View{Status: campaign.StatusActive},
			want: false,
		},
		{
			name: "Edge: a deleting campaign is refused",
			view: campaign.View{Status: campaign.StatusDeleting, TemplateIDs: []string{gateTemplateID}},
			want: false,
		},
		{
			name:     "Edge: a missing or foreign campaign is refused without an error",
			fetchErr: apperr.NewError(apperr.CodeCampaignNotFound, "campaign not found", nil),
			want:     false,
		},
		{
			name:     "Edge: a forbidden campaign is refused without an error",
			fetchErr: apperr.NewError(apperr.CodeCampaignForbidden, "campaign forbidden", nil),
			want:     false,
		},
		{
			name:     "Error: an unexpected lookup failure is returned",
			fetchErr: internalErr,
			want:     false,
			wantErr:  true,
		},
		{
			name:     "Error: an internal apperr lookup failure is returned",
			fetchErr: apperr.NewError(apperr.CodeInternal, "campaign read failed", internalErr),
			want:     false,
			wantErr:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := campaignmocks.NewMockService(t)
			service.EXPECT().
				Fetch(mock.Anything, gateCampaignID, gateUserID).
				Return(tc.view, tc.fetchErr).
				Once()

			gate := &campaignScopeGate{service: service}
			allowed, err := gate.AllowsTemplate(context.Background(), gateUserID, gateCampaignID, gateTemplateID)

			assert.Equal(t, tc.want, allowed)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCampaignScopeGate_UnboundServiceFails(t *testing.T) {
	gate := &campaignScopeGate{}

	allowed, err := gate.AllowsTemplate(context.Background(), gateUserID, gateCampaignID, gateTemplateID)

	require.Error(t, err)
	assert.False(t, allowed)
}
