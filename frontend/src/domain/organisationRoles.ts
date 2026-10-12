import type {
    AssignableOrgRole,
    OrganisationInviteResponse,
    OrganisationMemberResponse,
    OrgRole,
    StaffRole,
    TeamRole,
} from "../types/api";

export type OrgPermission =
    | "org.view"
    | "org.create_board"
    | "org.update"
    | "org.manage_members"
    | "org.manage_admins"
    | "org.manage_teams"
    | "org.manage_boards"
    | "org.manage_billing"
    | "org.view_audit_log"
    | "org.export"
    | "org.transfer_ownership"
    | "org.delete";

export type TeamPermission =
    | "team.view"
    | "team.update"
    | "team.manage_members"
    | "team.create_board"
    | "team.manage_boards"
    | "team.manage_action_items";

export interface RoleHolder {
    orgRole: OrgRole;
    teamRole?: TeamRole | null;
    siteRole?: StaffRole | null;
}

export interface OrgViewer {
    userId: string;
    role: OrgRole | null;
    siteRole: StaffRole | null;
}

export type OrgMemberTarget = Pick<OrganisationMemberResponse, "user_id" | "role" | "site_role">;

export interface OrgMemberActions {
    roles: AssignableOrgRole[];
    remove: boolean;
    transferOwnership: boolean;
}

export interface TeamViewer {
    userId: string;
    orgRole: OrgRole | null;
    teamRole: TeamRole | null;
    siteRole: StaffRole | null;
}

export interface TeamPerson {
    userId: string;
    orgRole: OrgRole;
    teamRole: TeamRole | null;
    siteRole: StaffRole | null;
}

export interface TeamMemberActions {
    roles: TeamRole[];
    remove: boolean;
}

export const ORG_ROLE_LABELS: Readonly<Record<OrgRole, string>> = {
    owner: "Owner",
    admin: "Admin",
    member: "Member",
};

export const TEAM_ROLE_LABELS: Readonly<Record<TeamRole, string>> = {
    lead: "Lead",
    member: "Member",
};

export const ASSIGNABLE_ORG_ROLES: readonly AssignableOrgRole[] = ["admin", "member"];

export const TEAM_ROLES: readonly TeamRole[] = ["lead", "member"];

const MEMBER_GRANTS: readonly OrgPermission[] = ["org.view", "org.create_board"];

const ADMIN_GRANTS: readonly OrgPermission[] = [
    ...MEMBER_GRANTS,
    "org.update",
    "org.manage_members",
    "org.manage_teams",
    "org.manage_boards",
    "org.manage_billing",
    "org.view_audit_log",
    "org.export",
];

const ORG_GRANTS: Readonly<Record<OrgRole, readonly OrgPermission[]>> = {
    member: MEMBER_GRANTS,
    admin: ADMIN_GRANTS,
    owner: [...ADMIN_GRANTS, "org.manage_admins", "org.transfer_ownership", "org.delete"],
};

const TEAM_MEMBER_GRANTS: readonly TeamPermission[] = ["team.view", "team.create_board", "team.manage_action_items"];

const TEAM_GRANTS: Readonly<Record<TeamRole, readonly TeamPermission[]>> = {
    member: TEAM_MEMBER_GRANTS,
    lead: [...TEAM_MEMBER_GRANTS, "team.update", "team.manage_members", "team.manage_boards"],
};

const ORG_RANKS: Readonly<Record<OrgRole, number>> = { member: 1, admin: 2, owner: 3 };

const TEAM_RANKS: Readonly<Record<TeamRole, number>> = { member: 1, lead: 2 };

const SITE_RANKS: Readonly<Record<StaffRole, number>> = { support: 1, admin: 2, super_admin: 3 };

const ORG_STEP = 10;

const SITE_STEP = 100;

const NO_MEMBER_ACTIONS: OrgMemberActions = { roles: [], remove: false, transferOwnership: false };

const NO_TEAM_MEMBER_ACTIONS: TeamMemberActions = { roles: [], remove: false };

export function orgRoleGrants(role: OrgRole | null | undefined, permission: OrgPermission): boolean {
    if (role === null || role === undefined) {
        return false;
    }

    return ORG_GRANTS[role].includes(permission);
}

export function effectiveTeamRole(orgRole: OrgRole, teamRole: TeamRole | null | undefined): TeamRole | null {
    if (orgRole === "owner" || orgRole === "admin") {
        return "lead";
    }

    return teamRole ?? null;
}

export function teamRoleGrants(
    orgRole: OrgRole | null | undefined,
    teamRole: TeamRole | null | undefined,
    permission: TeamPermission,
): boolean {
    if (orgRole === null || orgRole === undefined) {
        return false;
    }

    const effective = effectiveTeamRole(orgRole, teamRole);

    if (effective === null) {
        return false;
    }

    return TEAM_GRANTS[effective].includes(permission);
}

function level(holder: RoleHolder): number {
    if (holder.siteRole !== null && holder.siteRole !== undefined) {
        return SITE_RANKS[holder.siteRole] * SITE_STEP;
    }

    const team = effectiveTeamRole(holder.orgRole, holder.teamRole);
    const teamRank = team === null ? 0 : TEAM_RANKS[team];

    return ORG_RANKS[holder.orgRole] * ORG_STEP + teamRank;
}

export function outranks(actor: RoleHolder, target: RoleHolder): boolean {
    return level(actor) > level(target);
}

function mayChangeAdmins(viewerRole: OrgRole, from: OrgRole, to: OrgRole): boolean {
    if (from !== "admin" && to !== "admin") {
        return true;
    }

    return orgRoleGrants(viewerRole, "org.manage_admins");
}

export function orgMemberActions(viewer: OrgViewer, member: OrgMemberTarget): OrgMemberActions {
    const actor = viewer.role;

    if (actor === null || viewer.userId === member.user_id || member.role === "owner") {
        return NO_MEMBER_ACTIONS;
    }

    const holder: RoleHolder = { orgRole: actor, siteRole: viewer.siteRole };
    const siteRole = member.site_role ?? null;

    if (!outranks(holder, { orgRole: member.role, siteRole })) {
        return NO_MEMBER_ACTIONS;
    }

    const manages = orgRoleGrants(actor, "org.manage_members");

    const roles = manages
        ? ASSIGNABLE_ORG_ROLES.filter(
              role =>
                  role !== member.role &&
                  mayChangeAdmins(actor, member.role, role) &&
                  outranks(holder, { orgRole: role, siteRole }),
          )
        : [];

    return {
        roles,
        remove: manages && mayChangeAdmins(actor, member.role, member.role),
        transferOwnership: orgRoleGrants(actor, "org.transfer_ownership"),
    };
}

export function teamMemberActions(viewer: TeamViewer, member: TeamPerson): TeamMemberActions {
    const actorRole = viewer.orgRole;

    if (actorRole === null || viewer.userId === member.userId || member.teamRole === null) {
        return NO_TEAM_MEMBER_ACTIONS;
    }

    const actor: RoleHolder = { orgRole: actorRole, teamRole: viewer.teamRole, siteRole: viewer.siteRole };

    if (!teamRoleGrants(actorRole, viewer.teamRole, "team.manage_members") || !outranks(actor, member)) {
        return NO_TEAM_MEMBER_ACTIONS;
    }

    const roles = TEAM_ROLES.filter(role => role !== member.teamRole && outranks(actor, { ...member, teamRole: role }));

    return { roles, remove: true };
}

export function teamMemberAdditionRoles(viewer: TeamViewer, candidate: TeamPerson): TeamRole[] {
    const actorRole = viewer.orgRole;

    if (actorRole === null || viewer.userId === candidate.userId || candidate.teamRole !== null) {
        return [];
    }

    const actor: RoleHolder = { orgRole: actorRole, teamRole: viewer.teamRole, siteRole: viewer.siteRole };

    if (!teamRoleGrants(actorRole, viewer.teamRole, "team.manage_members") || !outranks(actor, candidate)) {
        return [];
    }

    return TEAM_ROLES.filter(role => outranks(actor, { ...candidate, teamRole: role }));
}

export function invitableRoles(role: OrgRole | null): AssignableOrgRole[] {
    if (!orgRoleGrants(role, "org.manage_members")) {
        return [];
    }

    return ASSIGNABLE_ORG_ROLES.filter(invited => invited === "member" || orgRoleGrants(role, "org.manage_admins"));
}

export function canManageInvite(role: OrgRole | null, invite: Pick<OrganisationInviteResponse, "role">): boolean {
    return invitableRoles(role).some(invitable => invitable === invite.role);
}

export function canLeaveOrganisation(role: OrgRole | null): boolean {
    return role !== null && role !== "owner";
}
