import { describe, expect, it } from "vitest";
import type { OrgRole, StaffRole, TeamRole } from "../types/api";
import {
    canLeaveOrganisation,
    canManageInvite,
    effectiveTeamRole,
    invitableRoles,
    ORG_ROLE_LABELS,
    orgMemberActions,
    orgRoleGrants,
    outranks,
    TEAM_ROLE_LABELS,
    teamMemberActions,
    teamMemberAdditionRoles,
    teamRoleGrants,
    type OrgPermission,
    type OrgViewer,
    type TeamPerson,
    type TeamViewer,
} from "./organisationRoles";

const ORG_PERMISSIONS: readonly OrgPermission[] = [
    "org.view",
    "org.create_board",
    "org.update",
    "org.manage_members",
    "org.manage_admins",
    "org.manage_teams",
    "org.manage_boards",
    "org.manage_billing",
    "org.view_audit_log",
    "org.export",
    "org.transfer_ownership",
    "org.delete",
];

function granted(role: OrgRole | null): OrgPermission[] {
    return ORG_PERMISSIONS.filter(permission => orgRoleGrants(role, permission));
}

function orgViewer(role: OrgRole | null, siteRole: StaffRole | null = null): OrgViewer {
    return { userId: "u1", role, siteRole };
}

function teamViewer(orgRole: OrgRole | null, teamRole: TeamRole | null, siteRole: StaffRole | null = null): TeamViewer {
    return { userId: "viewer", orgRole, teamRole, siteRole };
}

function teamPerson(orgRole: OrgRole, teamRole: TeamRole | null, siteRole: StaffRole | null = null): TeamPerson {
    return { userId: "target", orgRole, teamRole, siteRole };
}

describe("orgRoleGrants", () => {
    it("gives a member the view of the organisation and the right to create organisation boards", () => {
        // when
        const permissions = granted("member");

        // then
        expect(permissions).toEqual(["org.view", "org.create_board"]);
    });

    it("gives an admin everything but making admins, transferring ownership and deleting", () => {
        // when
        const permissions = granted("admin");

        // then
        expect(permissions).toEqual([
            "org.view",
            "org.create_board",
            "org.update",
            "org.manage_members",
            "org.manage_teams",
            "org.manage_boards",
            "org.manage_billing",
            "org.view_audit_log",
            "org.export",
        ]);
    });

    it("gives the owner every permission", () => {
        // when
        const permissions = granted("owner");

        // then
        expect(permissions).toEqual(ORG_PERMISSIONS);
    });

    it("grants nothing to someone with no role in the organisation", () => {
        // when
        const permissions = granted(null);

        // then
        expect(permissions).toEqual([]);
    });
});

describe("effectiveTeamRole", () => {
    it("counts owners and admins as lead on every team, on it or not", () => {
        // when
        const roles = [
            effectiveTeamRole("owner", null),
            effectiveTeamRole("admin", "member"),
            effectiveTeamRole("admin", undefined),
        ];

        // then
        expect(roles).toEqual(["lead", "lead", "lead"]);
    });

    it("keeps a member's own team role, or none when they are off the team", () => {
        // when
        const roles = [
            effectiveTeamRole("member", "lead"),
            effectiveTeamRole("member", "member"),
            effectiveTeamRole("member", null),
        ];

        // then
        expect(roles).toEqual(["lead", "member", null]);
    });
});

describe("teamRoleGrants", () => {
    it("lets a lead, an admin and the owner manage the team's members", () => {
        // when
        const allowed = [
            teamRoleGrants("member", "lead", "team.manage_members"),
            teamRoleGrants("admin", null, "team.manage_members"),
            teamRoleGrants("owner", null, "team.update"),
        ];

        // then
        expect(allowed).toEqual([true, true, true]);
    });

    it("lets a team member view the team and create boards but not manage it", () => {
        // when
        const allowed = [
            teamRoleGrants("member", "member", "team.view"),
            teamRoleGrants("member", "member", "team.create_board"),
            teamRoleGrants("member", "member", "team.update"),
            teamRoleGrants("member", "member", "team.manage_members"),
        ];

        // then
        expect(allowed).toEqual([true, true, false, false]);
    });

    it("grants nothing on the team to a member who is off it", () => {
        // when
        const allowed = teamRoleGrants("member", null, "team.view");

        // then
        expect(allowed).toBe(false);
    });
});

describe("outranks", () => {
    it("puts the owner above an admin and an admin above a member", () => {
        // when
        const ranks = [
            outranks({ orgRole: "owner" }, { orgRole: "admin" }),
            outranks({ orgRole: "admin" }, { orgRole: "member" }),
            outranks({ orgRole: "member" }, { orgRole: "admin" }),
        ];

        // then
        expect(ranks).toEqual([true, true, false]);
    });

    it("never lets anyone outrank someone at their own level", () => {
        // when
        const ranks = [
            outranks({ orgRole: "admin" }, { orgRole: "admin" }),
            outranks({ orgRole: "member", teamRole: "lead" }, { orgRole: "member", teamRole: "lead" }),
        ];

        // then
        expect(ranks).toEqual([false, false]);
    });

    it("puts a team lead above a member on the team and a member off it", () => {
        // when
        const ranks = [
            outranks({ orgRole: "member", teamRole: "lead" }, { orgRole: "member", teamRole: "member" }),
            outranks({ orgRole: "member", teamRole: "lead" }, { orgRole: "member", teamRole: null }),
        ];

        // then
        expect(ranks).toEqual([true, true]);
    });

    it("keeps a team lead below an admin, who counts as lead on every team", () => {
        // when
        const ranks = outranks({ orgRole: "member", teamRole: "lead" }, { orgRole: "admin", teamRole: null });

        // then
        expect(ranks).toBe(false);
    });

    it("puts site staff above every organisation role, whatever their own organisation role", () => {
        // when
        const ranks = [
            outranks({ orgRole: "member", siteRole: "support" }, { orgRole: "owner" }),
            outranks({ orgRole: "owner" }, { orgRole: "member", siteRole: "support" }),
        ];

        // then
        expect(ranks).toEqual([true, false]);
    });

    it("ranks staff by their site role alone, so two site admins stand level whoever owns the organisation", () => {
        // when
        const ranks = [
            outranks({ orgRole: "owner", siteRole: "admin" }, { orgRole: "member", siteRole: "admin" }),
            outranks({ orgRole: "member", siteRole: "admin" }, { orgRole: "owner", siteRole: "support" }),
            outranks({ orgRole: "owner", siteRole: "admin" }, { orgRole: "member", siteRole: "super_admin" }),
        ];

        // then
        expect(ranks).toEqual([false, true, false]);
    });
});

describe("orgMemberActions", () => {
    it("lets the owner make a member an admin, remove them and hand them the organisation", () => {
        // when
        const actions = orgMemberActions(orgViewer("owner"), { user_id: "u2", role: "member" });

        // then
        expect(actions).toEqual({ roles: ["admin"], remove: true, transferOwnership: true });
    });

    it("lets the owner make an admin a member again and remove them", () => {
        // when
        const actions = orgMemberActions(orgViewer("owner"), { user_id: "u2", role: "admin" });

        // then
        expect(actions).toEqual({ roles: ["member"], remove: true, transferOwnership: true });
    });

    it("lets an admin remove a member but never make an admin", () => {
        // when
        const actions = orgMemberActions(orgViewer("admin"), { user_id: "u2", role: "member" });

        // then
        expect(actions).toEqual({ roles: [], remove: true, transferOwnership: false });
    });

    it("shows an admin nothing to do to another admin, who stands on the same level", () => {
        // when
        const actions = orgMemberActions(orgViewer("admin"), { user_id: "u2", role: "admin" });

        // then
        expect(actions).toEqual({ roles: [], remove: false, transferOwnership: false });
    });

    it("shows nobody an action on the owner", () => {
        // when
        const actions = orgMemberActions(orgViewer("admin", "admin"), { user_id: "u2", role: "owner" });

        // then
        expect(actions).toEqual({ roles: [], remove: false, transferOwnership: false });
    });

    it("shows nobody an action on themselves", () => {
        // when
        const actions = orgMemberActions(orgViewer("owner"), { user_id: "u1", role: "owner" });

        // then
        expect(actions).toEqual({ roles: [], remove: false, transferOwnership: false });
    });

    it("shows a member nothing to do to anyone", () => {
        // when
        const actions = orgMemberActions(orgViewer("member"), { user_id: "u2", role: "member" });

        // then
        expect(actions).toEqual({ roles: [], remove: false, transferOwnership: false });
    });

    it("shows nothing to someone whose role is not known yet", () => {
        // when
        const actions = orgMemberActions(orgViewer(null), { user_id: "u2", role: "member" });

        // then
        expect(actions).toEqual({ roles: [], remove: false, transferOwnership: false });
    });

    it("shows the owner nothing to do to a member who is site staff, who stands above every organisation role", () => {
        // when
        const actions = orgMemberActions(orgViewer("owner"), { user_id: "u2", role: "member", site_role: "support" });

        // then
        expect(actions).toEqual({ roles: [], remove: false, transferOwnership: false });
    });

    it("keeps an admin who is site staff to what their organisation role allows", () => {
        // when
        const actions = [
            orgMemberActions(orgViewer("admin", "support"), { user_id: "u2", role: "member" }),
            orgMemberActions(orgViewer("admin", "support"), { user_id: "u3", role: "admin" }),
        ];

        // then
        expect(actions).toEqual([
            { roles: [], remove: true, transferOwnership: false },
            { roles: [], remove: false, transferOwnership: false },
        ]);
    });

    it("lets an owner who is site staff act on staff below them, but not on staff at their level", () => {
        // when
        const actions = [
            orgMemberActions(orgViewer("owner", "admin"), { user_id: "u2", role: "member", site_role: "support" }),
            orgMemberActions(orgViewer("owner", "admin"), { user_id: "u3", role: "member", site_role: "admin" }),
        ];

        // then
        expect(actions).toEqual([
            { roles: ["admin"], remove: true, transferOwnership: true },
            { roles: [], remove: false, transferOwnership: false },
        ]);
    });
});

describe("teamMemberActions", () => {
    it("lets a lead remove a member of the team but not make them a lead, which would put them level", () => {
        // when
        const actions = teamMemberActions(teamViewer("member", "lead"), teamPerson("member", "member"));

        // then
        expect(actions).toEqual({ roles: [], remove: true });
    });

    it("lets an admin make a member a lead and remove them", () => {
        // when
        const actions = teamMemberActions(teamViewer("admin", null), teamPerson("member", "member"));

        // then
        expect(actions).toEqual({ roles: ["lead"], remove: true });
    });

    it("lets an admin make a lead a member again", () => {
        // when
        const actions = teamMemberActions(teamViewer("admin", null), teamPerson("member", "lead"));

        // then
        expect(actions).toEqual({ roles: ["member"], remove: true });
    });

    it("shows a lead nothing to do to another lead", () => {
        // when
        const actions = teamMemberActions(teamViewer("member", "lead"), teamPerson("member", "lead"));

        // then
        expect(actions).toEqual({ roles: [], remove: false });
    });

    it("shows a lead nothing to do to an admin on the team", () => {
        // when
        const actions = teamMemberActions(teamViewer("member", "lead"), teamPerson("admin", "member"));

        // then
        expect(actions).toEqual({ roles: [], remove: false });
    });

    it("shows a team member nothing to do to anyone", () => {
        // when
        const actions = teamMemberActions(teamViewer("member", "member"), teamPerson("member", "member"));

        // then
        expect(actions).toEqual({ roles: [], remove: false });
    });

    it("shows nothing for themselves or for someone off the team", () => {
        // given
        const viewer = teamViewer("owner", null);

        // when
        const actions = [
            teamMemberActions(viewer, { userId: "viewer", orgRole: "member", teamRole: "member", siteRole: null }),
            teamMemberActions(viewer, teamPerson("member", null)),
        ];

        // then
        expect(actions).toEqual([
            { roles: [], remove: false },
            { roles: [], remove: false },
        ]);
    });

    it("shows the owner nothing to do to a team member who is site staff", () => {
        // when
        const actions = teamMemberActions(teamViewer("owner", null), teamPerson("member", "member", "support"));

        // then
        expect(actions).toEqual({ roles: [], remove: false });
    });

    it("lets a lead who is site staff make a member a lead, since the change still leaves them below", () => {
        // when
        const actions = teamMemberActions(teamViewer("member", "lead", "support"), teamPerson("member", "member"));

        // then
        expect(actions).toEqual({ roles: ["lead"], remove: true });
    });
});

describe("teamMemberAdditionRoles", () => {
    it("lets a lead add a member of the organisation as a team member only", () => {
        // when
        const roles = teamMemberAdditionRoles(teamViewer("member", "lead"), teamPerson("member", null));

        // then
        expect(roles).toEqual(["member"]);
    });

    it("lets an admin add a member of the organisation as a lead or a member", () => {
        // when
        const roles = teamMemberAdditionRoles(teamViewer("admin", null), teamPerson("member", null));

        // then
        expect(roles).toEqual(["lead", "member"]);
    });

    it("keeps a lead from adding an admin, who already outranks them", () => {
        // when
        const roles = teamMemberAdditionRoles(teamViewer("member", "lead"), teamPerson("admin", null));

        // then
        expect(roles).toEqual([]);
    });

    it("offers nothing for someone already on the team, or to a team member", () => {
        // when
        const roles = [
            teamMemberAdditionRoles(teamViewer("owner", null), teamPerson("member", "member")),
            teamMemberAdditionRoles(teamViewer("member", "member"), teamPerson("member", null)),
        ];

        // then
        expect(roles).toEqual([[], []]);
    });

    it("keeps the owner from adding site staff, who stand above them", () => {
        // when
        const roles = teamMemberAdditionRoles(teamViewer("owner", null), teamPerson("member", null, "admin"));

        // then
        expect(roles).toEqual([]);
    });
});

describe("invitableRoles", () => {
    it("lets the owner invite members and admins", () => {
        // when
        const roles = invitableRoles("owner");

        // then
        expect(roles).toEqual(["admin", "member"]);
    });

    it("lets an admin invite members only", () => {
        // when
        const roles = invitableRoles("admin");

        // then
        expect(roles).toEqual(["member"]);
    });

    it("lets a member invite nobody", () => {
        // when
        const roles = [invitableRoles("member"), invitableRoles(null)];

        // then
        expect(roles).toEqual([[], []]);
    });
});

describe("canManageInvite", () => {
    it("lets the owner resend or revoke any invite", () => {
        // when
        const allowed = [canManageInvite("owner", { role: "admin" }), canManageInvite("owner", { role: "member" })];

        // then
        expect(allowed).toEqual([true, true]);
    });

    it("lets an admin resend or revoke a member invite but not an admin invite", () => {
        // when
        const allowed = [canManageInvite("admin", { role: "member" }), canManageInvite("admin", { role: "admin" })];

        // then
        expect(allowed).toEqual([true, false]);
    });

    it("lets a member manage no invite", () => {
        // when
        const allowed = [canManageInvite("member", { role: "member" }), canManageInvite(null, { role: "member" })];

        // then
        expect(allowed).toEqual([false, false]);
    });
});

describe("canLeaveOrganisation", () => {
    it("lets admins and members leave, but not the owner", () => {
        // when
        const allowed = [
            canLeaveOrganisation("admin"),
            canLeaveOrganisation("member"),
            canLeaveOrganisation("owner"),
            canLeaveOrganisation(null),
        ];

        // then
        expect(allowed).toEqual([true, true, false, false]);
    });
});

describe("role labels", () => {
    it("names every organisation and team role", () => {
        // then
        expect(ORG_ROLE_LABELS).toEqual({ owner: "Owner", admin: "Admin", member: "Member" });
        expect(TEAM_ROLE_LABELS).toEqual({ lead: "Lead", member: "Member" });
    });
});
