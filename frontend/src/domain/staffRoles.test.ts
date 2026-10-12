import { describe, expect, it } from "vitest";
import type { StaffPermission } from "./permissions";
import {
    GRANTABLE_STAFF_ROLES,
    isGrantableStaffRole,
    samePermissions,
    staffActionBlock,
    staffRoleLabel,
    withPermission,
} from "./staffRoles";

const adminPermissions: StaffPermission[] = ["view_admin_panel", "view_users", "ban_user", "manage_roles"];
const supportPermissions: StaffPermission[] = ["view_admin_panel", "view_users", "ban_user"];

describe("staffActionBlock", () => {
    it("lets staff act on an ordinary user", () => {
        // given
        const target = { id: "user-2" };

        // when
        const block = staffActionBlock("user-1", supportPermissions, target);

        // then
        expect(block).toBeNull();
    });

    it("stops staff acting on their own account", () => {
        // given
        const target = { id: "user-1", role: "admin" as const };

        // when
        const block = staffActionBlock("user-1", adminPermissions, target);

        // then
        expect(block).toBe("self");
    });

    it("stops anyone acting on a super admin", () => {
        // given
        const target = { id: "user-2", role: "super_admin" as const };

        // when
        const block = staffActionBlock("user-1", adminPermissions, target);

        // then
        expect(block).toBe("super_admin");
    });

    it("stops a support member acting on other staff, since only admins manage roles", () => {
        // given
        const target = { id: "user-2", role: "support" as const };

        // when
        const block = staffActionBlock("user-1", supportPermissions, target);

        // then
        expect(block).toBe("outranked");
    });

    it("leaves an admin's action on other staff to the server's rank check", () => {
        // given
        const target = { id: "user-2", role: "admin" as const };

        // when
        const block = staffActionBlock("user-1", adminPermissions, target);

        // then
        expect(block).toBeNull();
    });
});

describe("staffRoleLabel", () => {
    it("names each staff role", () => {
        // given
        const roles = ["super_admin", "admin", "support"] as const;

        // when
        const labels = roles.map(role => staffRoleLabel(role));

        // then
        expect(labels).toEqual(["Super admin", "Admin", "Support"]);
    });

    it("says so when the user holds no role", () => {
        // given
        const role = undefined;

        // when
        const label = staffRoleLabel(role);

        // then
        expect(label).toBe("No role");
    });
});

describe("withPermission", () => {
    it("adds a permission once", () => {
        // given
        const granted = ["view_users"];

        // when
        const added = withPermission(granted, "ban_user", true);
        const again = withPermission(added, "ban_user", true);

        // then
        expect(added).toEqual(["view_users", "ban_user"]);
        expect(again).toEqual(["view_users", "ban_user"]);
    });

    it("takes a permission away", () => {
        // given
        const granted = ["view_users", "ban_user"];

        // when
        const removed = withPermission(granted, "view_users", false);

        // then
        expect(removed).toEqual(["ban_user"]);
    });
});

describe("samePermissions", () => {
    it("treats the same permissions in any order as the same", () => {
        // given
        const saved = ["ban_user", "view_users"];
        const edited = ["view_users", "ban_user"];

        // when
        const same = samePermissions(saved, edited);

        // then
        expect(same).toBe(true);
    });

    it("tells a different set apart", () => {
        // given
        const saved = ["ban_user", "view_users"];

        // when
        const results = [
            samePermissions(saved, ["view_users"]),
            samePermissions(saved, ["view_users", "reset_password"]),
        ];

        // then
        expect(results).toEqual([false, false]);
    });
});

describe("isGrantableStaffRole", () => {
    it("accepts the roles staff can hand out", () => {
        // given
        const values = ["support", "admin"];

        // when
        const grantable = values.map(value => isGrantableStaffRole(value));

        // then
        expect(grantable).toEqual([true, true]);
    });

    it("refuses super admin, no role and anything unknown", () => {
        // given
        const values = ["super_admin", undefined, "moderator"];

        // when
        const grantable = values.map(value => isGrantableStaffRole(value));

        // then
        expect(grantable).toEqual([false, false, false]);
    });
});

describe("GRANTABLE_STAFF_ROLES", () => {
    it("never offers super admin, which no staff member can grant", () => {
        // given
        const offered = GRANTABLE_STAFF_ROLES;

        // when
        const includesSuperAdmin = offered.includes("super_admin");

        // then
        expect(includesSuperAdmin).toBe(false);
        expect(offered).toEqual(["support", "admin"]);
    });
});
