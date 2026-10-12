package orgauthz

import (
	"slices"

	"retrotide/internal/model"
)

type (
	Permission = model.TenantPermission

	PermissionScope string

	PermissionDef struct {
		Permission Permission
		Label      string
		Scope      PermissionScope
	}
)

const (
	ScopeOrg  PermissionScope = "org"
	ScopeTeam PermissionScope = "team"
)

const (
	PermOrgView              Permission = "org.view"
	PermOrgUpdate            Permission = "org.update"
	PermOrgManageMembers     Permission = "org.manage_members"
	PermOrgManageAdmins      Permission = "org.manage_admins"
	PermOrgManageTeams       Permission = "org.manage_teams"
	PermOrgCreateBoard       Permission = "org.create_board"
	PermOrgManageBoards      Permission = "org.manage_boards"
	PermOrgManageBilling     Permission = "org.manage_billing"
	PermOrgViewAuditLog      Permission = "org.view_audit_log"
	PermOrgExport            Permission = "org.export"
	PermOrgTransferOwnership Permission = "org.transfer_ownership"
	PermOrgDelete            Permission = "org.delete"

	PermTeamView              Permission = "team.view"
	PermTeamUpdate            Permission = "team.update"
	PermTeamManageMembers     Permission = "team.manage_members"
	PermTeamCreateBoard       Permission = "team.create_board"
	PermTeamManageBoards      Permission = "team.manage_boards"
	PermTeamManageActionItems Permission = "team.manage_action_items"
)

var (
	permissionCatalogue = []PermissionDef{
		{PermOrgView, "View the organisation", ScopeOrg},
		{PermOrgUpdate, "Edit the organisation", ScopeOrg},
		{PermOrgManageMembers, "Manage members and invites", ScopeOrg},
		{PermOrgManageAdmins, "Make and unmake admins", ScopeOrg},
		{PermOrgManageTeams, "Manage teams", ScopeOrg},
		{PermOrgCreateBoard, "Create organisation boards", ScopeOrg},
		{PermOrgManageBoards, "Manage the organisation's boards", ScopeOrg},
		{PermOrgManageBilling, "Manage billing", ScopeOrg},
		{PermOrgViewAuditLog, "View the audit log", ScopeOrg},
		{PermOrgExport, "Export the organisation", ScopeOrg},
		{PermOrgTransferOwnership, "Transfer ownership", ScopeOrg},
		{PermOrgDelete, "Delete the organisation", ScopeOrg},
		{PermTeamView, "View the team", ScopeTeam},
		{PermTeamUpdate, "Edit the team", ScopeTeam},
		{PermTeamManageMembers, "Manage team members", ScopeTeam},
		{PermTeamCreateBoard, "Create boards", ScopeTeam},
		{PermTeamManageBoards, "Manage the team's boards", ScopeTeam},
		{PermTeamManageActionItems, "Manage action items", ScopeTeam},
	}

	orgMemberGrants = []Permission{PermOrgView, PermOrgCreateBoard}
	orgAdminGrants  = []Permission{PermOrgUpdate, PermOrgManageMembers, PermOrgManageTeams, PermOrgManageBoards, PermOrgManageBilling, PermOrgViewAuditLog, PermOrgExport}
	orgOwnerGrants  = []Permission{PermOrgManageAdmins, PermOrgTransferOwnership, PermOrgDelete}

	teamMemberGrants = []Permission{PermTeamView, PermTeamCreateBoard, PermTeamManageActionItems}
	teamLeadGrants   = []Permission{PermTeamUpdate, PermTeamManageMembers, PermTeamManageBoards}

	orgRoleGrants = map[model.OrgRole][]Permission{
		model.OrgRoleMember: orgMemberGrants,
		model.OrgRoleAdmin:  slices.Concat(orgMemberGrants, orgAdminGrants),
		model.OrgRoleOwner:  slices.Concat(orgMemberGrants, orgAdminGrants, orgOwnerGrants),
	}

	teamRoleGrants = map[model.TeamRole][]Permission{
		model.TeamRoleMember: teamMemberGrants,
		model.TeamRoleLead:   slices.Concat(teamMemberGrants, teamLeadGrants),
	}

	permissionIndex = buildPermissionIndex()
)

func buildPermissionIndex() map[Permission]PermissionDef {
	index := make(map[Permission]PermissionDef, len(permissionCatalogue))
	for _, def := range permissionCatalogue {
		index[def.Permission] = def
	}

	return index
}

func LookupPermission(perm Permission) (PermissionDef, bool) {
	def, ok := permissionIndex[perm]

	return def, ok
}

func OrgRoleGrants(r model.OrgRole, perm Permission) bool {
	granted := orgRoleGrants[r]

	return slices.Contains(granted, perm)
}

func TeamRoleGrants(r model.TeamRole, perm Permission) bool {
	granted := teamRoleGrants[r]

	return slices.Contains(granted, perm)
}

func EffectiveTeamRole(org model.OrgRole, team model.TeamRole) model.TeamRole {
	if org == model.OrgRoleOwner || org == model.OrgRoleAdmin {
		return model.TeamRoleLead
	}

	return team
}
