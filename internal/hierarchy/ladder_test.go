package hierarchy

import (
	"testing"

	"retrotide/internal/model"
	"retrotide/internal/role"

	"github.com/stretchr/testify/assert"
)

type (
	rung struct {
		name   string
		holder Holder
	}
)

var (
	nobody      = Holder{}
	support     = Holder{Site: role.Support}
	siteAdmin   = Holder{Site: role.Admin}
	superAdmin  = Holder{Site: role.SuperAdmin}
	owner       = Holder{Org: model.OrgRoleOwner}
	orgAdmin    = Holder{Org: model.OrgRoleAdmin}
	member      = Holder{Org: model.OrgRoleMember}
	lead        = Holder{Org: model.OrgRoleMember, Team: model.TeamRoleLead}
	teamMember  = Holder{Org: model.OrgRoleMember, Team: model.TeamRoleMember}
	supportPeer = Holder{Site: role.Support, Org: model.OrgRoleMember}

	ladderTopDown = []rung{
		{name: "the super admin", holder: superAdmin},
		{name: "a site admin", holder: siteAdmin},
		{name: "support", holder: support},
		{name: "the owner", holder: owner},
		{name: "an organisation admin", holder: orgAdmin},
		{name: "a team lead", holder: lead},
		{name: "a team member", holder: teamMember},
		{name: "a member on no team", holder: member},
		{name: "someone outside the organisation", holder: nobody},
	}
)

func TestStanding_EveryRungActsOnlyOnTheRungsBelowIt(t *testing.T) {
	for actorRung, actor := range ladderTopDown {
		for targetRung, target := range ladderTopDown {
			t.Run(actor.name+" on "+target.name, func(t *testing.T) {
				// given the ladder is listed from the top, so a lower index is a higher rung
				standing := Standing{Actor: actor.holder, Target: target.holder}
				want := actorRung < targetRung

				// when
				got := standing.Permits()

				// then
				assert.Equal(t, want, got)
			})
		}
	}
}

func TestStanding_StaffSitOnTheirSiteRungWhateverTheirOrganisationRole(t *testing.T) {
	tests := []struct {
		name   string
		actor  Holder
		target Holder
		want   bool
	}{
		{name: "support who owns the organisation on support who is a member", actor: Holder{Site: role.Support, Org: model.OrgRoleOwner}, target: supportPeer, want: false},
		{name: "support who is a member on support who owns the organisation", actor: supportPeer, target: Holder{Site: role.Support, Org: model.OrgRoleOwner}, want: false},
		{name: "a site admin who owns the organisation on a site admin who is a member", actor: Holder{Site: role.Admin, Org: model.OrgRoleOwner}, target: Holder{Site: role.Admin, Org: model.OrgRoleMember}, want: false},
		{name: "a site admin who is a member on support who owns the organisation", actor: Holder{Site: role.Admin, Org: model.OrgRoleMember}, target: Holder{Site: role.Support, Org: model.OrgRoleOwner}, want: true},
		{name: "support who leads a team on the owner", actor: Holder{Site: role.Support, Org: model.OrgRoleMember, Team: model.TeamRoleLead}, target: owner, want: true},
		{name: "the owner on support who is a team member", actor: owner, target: Holder{Site: role.Support, Org: model.OrgRoleMember, Team: model.TeamRoleMember}, want: false},
		{name: "the super admin on the super admin who is a member", actor: Holder{Site: role.SuperAdmin, Org: model.OrgRoleOwner}, target: Holder{Site: role.SuperAdmin, Org: model.OrgRoleMember}, want: false},
		{name: "an unknown site role counts as none", actor: Holder{Site: "moderator", Org: model.OrgRoleMember}, target: member, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			standing := Standing{Actor: tc.actor, Target: tc.target}

			// when
			got := standing.Permits()

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStanding_OnlyAStrictlyHigherLevelMayAct(t *testing.T) {
	tests := []struct {
		name   string
		actor  Holder
		target Holder
		want   bool
	}{
		{name: "support on an ordinary account", actor: support, target: nobody, want: true},
		{name: "a site admin on support", actor: siteAdmin, target: support, want: true},
		{name: "the super admin on a site admin", actor: superAdmin, target: siteAdmin, want: true},
		{name: "support on support", actor: support, target: support, want: false},
		{name: "support on a site admin", actor: support, target: siteAdmin, want: false},
		{name: "a site admin on a site admin", actor: siteAdmin, target: siteAdmin, want: false},
		{name: "a site admin on the super admin", actor: siteAdmin, target: superAdmin, want: false},
		{name: "the super admin on the super admin", actor: superAdmin, target: superAdmin, want: false},
		{name: "an account with no role on another", actor: nobody, target: nobody, want: false},

		{name: "an owner on an admin", actor: owner, target: orgAdmin, want: true},
		{name: "an owner on a member", actor: owner, target: member, want: true},
		{name: "an admin on a member", actor: orgAdmin, target: member, want: true},
		{name: "an admin on an admin", actor: orgAdmin, target: orgAdmin, want: false},
		{name: "an admin on the owner", actor: orgAdmin, target: owner, want: false},
		{name: "a member on a member", actor: member, target: member, want: false},
		{name: "a member on an admin", actor: member, target: orgAdmin, want: false},
		{name: "someone outside the organisation on a member", actor: nobody, target: member, want: false},

		{name: "site staff sit above the owner", actor: supportPeer, target: owner, want: true},
		{name: "the owner cannot act on site staff", actor: owner, target: supportPeer, want: false},

		{name: "a team lead on a team member", actor: lead, target: teamMember, want: true},
		{name: "a team lead on another lead", actor: lead, target: lead, want: false},
		{name: "a team lead on an admin, who leads every team", actor: lead, target: orgAdmin, want: false},
		{name: "an admin on a team lead", actor: orgAdmin, target: lead, want: true},
		{name: "a team member on a team member", actor: teamMember, target: teamMember, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			standing := Standing{Actor: tc.actor, Target: tc.target}

			// when
			got := standing.Permits()

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStanding_ARoleChangeMustLeaveTheTargetBelowTheActor(t *testing.T) {
	tests := []struct {
		name   string
		actor  Holder
		target Holder
		after  Holder
		want   bool
	}{
		{name: "a site admin grants support", actor: siteAdmin, target: nobody, after: support, want: true},
		{name: "a site admin grants admin", actor: siteAdmin, target: nobody, after: siteAdmin, want: false},
		{name: "the super admin grants admin", actor: superAdmin, target: support, after: siteAdmin, want: true},
		{name: "the super admin grants super admin", actor: superAdmin, target: siteAdmin, after: superAdmin, want: false},

		{name: "the owner makes a member an admin", actor: owner, target: member, after: orgAdmin, want: true},
		{name: "an admin makes a member an admin", actor: orgAdmin, target: member, after: orgAdmin, want: false},
		{name: "the owner makes a member the owner", actor: owner, target: member, after: owner, want: false},
		{name: "the owner makes site staff an admin", actor: owner, target: supportPeer, after: Holder{Site: role.Support, Org: model.OrgRoleAdmin}, want: false},
		{name: "the owner makes an admin a member", actor: owner, target: orgAdmin, after: member, want: true},
		{name: "an admin makes another admin a member", actor: orgAdmin, target: orgAdmin, after: member, want: false},
		{name: "a site admin makes support a member of the organisation", actor: Holder{Site: role.Admin, Org: model.OrgRoleMember}, target: supportPeer, after: Holder{Site: role.Support, Org: model.OrgRoleAdmin}, want: true},

		{name: "an admin makes a team member lead", actor: orgAdmin, target: teamMember, after: lead, want: true},
		{name: "a team lead makes a team member lead", actor: lead, target: teamMember, after: lead, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			standing := Standing{Actor: tc.actor, Target: tc.target}

			// when
			got := standing.MayChangeTo(tc.after)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
