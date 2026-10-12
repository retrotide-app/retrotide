package live

import (
	"testing"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMembershipRole_AnswersWhatMembershipAloneGives(t *testing.T) {
	tests := []struct {
		name     string
		header   model.BoardHeader
		standing teamStanding
		want     model.BoardRole
	}{
		{name: "a team lead", header: liveHeader(), standing: teamLead, want: model.BoardRoleFacilitator},
		{name: "a team member", header: liveHeader(), standing: teamMember, want: model.BoardRoleParticipant},
		{name: "an organisation member off the team", header: liveHeader(), standing: offTeam},
		{name: "an outsider", header: liveHeader(), standing: notInOrg},
		{name: "an organisation admin on an organisation board", header: organisationHeader(), standing: teamLead, want: model.BoardRoleFacilitator},
		{name: "an organisation member on an organisation board", header: organisationHeader(), standing: teamMember, want: model.BoardRoleParticipant},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			live, m := newTestLive(t)
			principal := user()
			m.standing(tc.header, principal, tc.standing)

			// when
			got, err := live.MembershipRole(t.Context(), tc.header, principal)

			// then no stored row is read, so a promotion never counts as membership
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMembershipRole_AGuestHoldsNoMembership(t *testing.T) {
	// given
	live, _ := newTestLive(t)

	// when
	got, err := live.MembershipRole(t.Context(), liveHeader(), model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()})

	// then the strict mocks prove nobody was asked
	require.NoError(t, err)
	assert.Empty(t, got)
}
