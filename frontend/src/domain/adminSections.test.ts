import { describe, expect, it } from "vitest";
import { makeOrganisation } from "../test-utils/fixtures";
import { ADMIN_SECTIONS, appReturnPath, groupedAdminSections, visibleAdminSections } from "./adminSections";
import { STAFF_PERMISSIONS, type StaffPermission } from "./permissions";

describe("appReturnPath", () => {
    it("goes back to the viewer's current organisation's boards", () => {
        // given
        const current = makeOrganisation({ slug: "harbour" });

        // when
        const path = appReturnPath(current);

        // then
        expect(path).toBe("/o/harbour");
    });

    it("goes to the account page when the viewer belongs to no organisation", () => {
        // when
        const path = appReturnPath(null);

        // then
        expect(path).toBe("/account");
    });

    it("goes to the account page when the current organisation has been deleted", () => {
        // given
        const current = makeOrganisation({ slug: "harbour", deleted_at: "2026-10-01T00:00:00Z" });

        // when
        const path = appReturnPath(current);

        // then
        expect(path).toBe("/account");
    });
});

describe("ADMIN_SECTIONS", () => {
    it("guards each admin page with the permission its API routes need", () => {
        // when
        const guards = ADMIN_SECTIONS.map(section => [section.path, section.permission]);

        // then
        expect(guards).toEqual([
            ["/admin", "view_admin_panel"],
            ["/admin/users", "view_users"],
            ["/admin/organisations", "view_organisations"],
            ["/admin/invitations", "manage_user_account"],
            ["/admin/permissions", "manage_roles"],
            ["/admin/settings", "manage_settings"],
            ["/admin/audit-log", "view_audit_log"],
        ]);
    });
});

describe("visibleAdminSections", () => {
    it("shows every section to someone who holds every permission on a private site", () => {
        // given
        const granted = STAFF_PERMISSIONS;

        // when
        const labels = visibleAdminSections(granted, "private").map(section => section.label);

        // then
        expect(labels).toEqual([
            "Dashboard",
            "Users",
            "Organisations",
            "Invitations",
            "Permissions",
            "Settings",
            "Audit log",
        ]);
    });

    it("leaves out the invitations on a public site, where nobody needs one to join", () => {
        // given
        const granted = STAFF_PERMISSIONS;

        // when
        const labels = visibleAdminSections(granted, "public").map(section => section.label);

        // then
        expect(labels).toEqual(["Dashboard", "Users", "Organisations", "Permissions", "Settings", "Audit log"]);
    });

    it("shows only the sections a support role may use", () => {
        // given
        const granted: StaffPermission[] = [
            "view_admin_panel",
            "view_stats",
            "view_users",
            "ban_user",
            "manage_user_account",
            "view_organisations",
        ];

        // when
        const labels = visibleAdminSections(granted, "private").map(section => section.label);

        // then
        expect(labels).toEqual(["Dashboard", "Users", "Organisations", "Invitations"]);
    });

    it("shows the invitations only to someone who may manage user accounts", () => {
        // given
        const granted: StaffPermission[] = ["view_admin_panel", "view_users"];

        // when
        const labels = visibleAdminSections(granted, "private").map(section => section.label);

        // then
        expect(labels).toEqual(["Dashboard", "Users"]);
    });

    it("shows nothing to someone with no staff permissions", () => {
        // given
        const granted: StaffPermission[] = [];

        // when
        const sections = visibleAdminSections(granted, "private");

        // then
        expect(sections).toEqual([]);
    });
});

describe("groupedAdminSections", () => {
    it("sorts the sections into the overview, the accounts and the site", () => {
        // given
        const granted = STAFF_PERMISSIONS;

        // when
        const groups = groupedAdminSections(granted, "private").map(view => [
            view.label,
            view.sections.map(section => section.label),
        ]);

        // then
        expect(groups).toEqual([
            ["Overview", ["Dashboard"]],
            ["Accounts", ["Users", "Organisations", "Invitations"]],
            ["Site", ["Permissions", "Settings", "Audit log"]],
        ]);
    });

    it("leaves out a group the viewer may open nothing in", () => {
        // given
        const granted: StaffPermission[] = ["view_admin_panel", "view_audit_log"];

        // when
        const labels = groupedAdminSections(granted, "public").map(view => view.label);

        // then
        expect(labels).toEqual(["Overview", "Site"]);
    });
});
