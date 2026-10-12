package hierarchy

import (
	"retrotide/internal/model"
	"retrotide/internal/orgauthz"
	"retrotide/internal/role"
)

type (
	level int

	Holder struct {
		Site role.Role
		Org  model.OrgRole
		Team model.TeamRole
	}

	Standing struct {
		Actor  Holder
		Target Holder
	}
)

const (
	siteStep = 100
	orgStep  = 10
)

var (
	orgRanks = map[model.OrgRole]int{
		model.OrgRoleMember: 1,
		model.OrgRoleAdmin:  2,
		model.OrgRoleOwner:  3,
	}

	teamRanks = map[model.TeamRole]int{
		model.TeamRoleMember: 1,
		model.TeamRoleLead:   2,
	}
)

func (h Holder) level() level {
	if h.Site.IsSiteStaff() {
		return level(h.Site.Rank() * siteStep)
	}

	team := orgauthz.EffectiveTeamRole(h.Org, h.Team)

	return level(orgRanks[h.Org]*orgStep + teamRanks[team])
}

func (s Standing) Permits() bool {
	if s.Target.Site == role.SuperAdmin {
		return false
	}

	actor, target := s.Actor.level(), s.Target.level()

	return actor > target
}

func (s Standing) MayChangeTo(after Holder) bool {
	if !s.Permits() {
		return false
	}

	actor, changed := s.Actor.level(), after.level()

	return actor > changed
}
