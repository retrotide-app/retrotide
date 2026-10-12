import { describe, expect, it } from "vitest";
import { STAFF_PERMISSIONS, staffPermissions } from "./permissions";

describe("STAFF_PERMISSIONS", () => {
    it("lists the staff catalogue in the order the server keeps it", () => {
        // given
        const catalogue = [
            "view_admin_panel",
            "view_stats",
            "view_audit_log",
            "manage_settings",
            "manage_roles",
            "view_users",
            "ban_user",
            "manage_user_account",
            "manage_user_email",
            "set_email_verified",
            "reset_password",
            "delete_any_user",
            "view_organisations",
            "grant_entitlements",
        ];

        // when
        const listed = [...STAFF_PERMISSIONS];

        // then
        expect(listed).toEqual(catalogue);
    });
});

describe("staffPermissions", () => {
    it("keeps every permission the catalogue knows", () => {
        // given
        const granted = ["view_admin_panel", "view_users", "ban_user"];

        // when
        const known = staffPermissions(granted);

        // then
        expect(known).toEqual(["view_admin_panel", "view_users", "ban_user"]);
    });

    it("drops a permission the catalogue does not know", () => {
        // given
        const granted = ["view_users", "fly_the_ship"];

        // when
        const known = staffPermissions(granted);

        // then
        expect(known).toEqual(["view_users"]);
    });

    it("puts the permissions in catalogue order whatever order they arrive in", () => {
        // given
        const granted = ["manage_settings", "view_admin_panel"];

        // when
        const known = staffPermissions(granted);

        // then
        expect(known).toEqual(["view_admin_panel", "manage_settings"]);
    });

    it("returns nothing for an account with no staff permissions", () => {
        // given
        const granted: string[] = [];

        // when
        const known = staffPermissions(granted);

        // then
        expect(known).toEqual([]);
    });
});
