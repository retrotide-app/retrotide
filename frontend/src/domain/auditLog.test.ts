import { describe, expect, it } from "vitest";
import { makeAuditLogEntry } from "../test-utils/fixtures";
import {
    AUDIT_ACTION_LABELS,
    auditActionLabel,
    auditActorName,
    auditDetailLabel,
    auditDetails,
    isAuditAction,
    SITE_AUDIT_ACTIONS,
} from "./auditLog";

describe("AUDIT_ACTION_LABELS", () => {
    it("names the staff actions in plain words", () => {
        // given
        const actions = ["ban_user", "reset_password", "update_role_permissions"] as const;

        // when
        const labels = actions.map(action => AUDIT_ACTION_LABELS[action]);

        // then
        expect(labels).toEqual(["User banned", "Password reset", "Role permissions updated"]);
    });

    it("names the invitation actions in plain words", () => {
        // given
        const actions = ["invite_user", "resend_invitation", "cancel_invitation", "invitation_accepted"] as const;

        // when
        const labels = actions.map(action => AUDIT_ACTION_LABELS[action]);

        // then
        expect(labels).toEqual([
            "User invited",
            "Invitation sent again",
            "Invitation cancelled",
            "Invitation accepted",
        ]);
    });

    it("names the two-factor actions in plain words", () => {
        // given
        const actions = ["two_factor_enabled", "two_factor_disabled"] as const;

        // when
        const labels = actions.map(action => AUDIT_ACTION_LABELS[action]);

        // then
        expect(labels).toEqual(["Two-factor authentication turned on", "Two-factor authentication turned off"]);
    });
});

describe("SITE_AUDIT_ACTIONS", () => {
    it("lists every action the site audit log records, once each", () => {
        // given
        const listed = SITE_AUDIT_ACTIONS;

        // when
        const distinct = new Set(listed);

        // then
        expect(distinct.size).toBe(listed.length);
        expect(listed).toContain("identity_linked");
        expect(listed).toContain("send_test_email");
        expect(listed).toContain("invite_user");
        expect(listed).toContain("invitation_accepted");
        expect(listed).toContain("two_factor_enabled");
        expect(listed).toContain("two_factor_disabled");
        expect(listed).not.toContain("board_created");
    });
});

describe("isAuditAction", () => {
    it("accepts a known action", () => {
        // given
        const value = "lock_user";

        // when
        const known = isAuditAction(value);

        // then
        expect(known).toBe(true);
    });

    it("refuses anything else, including inherited object keys", () => {
        // given
        const values = ["", "drop_tables", "toString"];

        // when
        const known = values.map(value => isAuditAction(value));

        // then
        expect(known).toEqual([false, false, false]);
    });
});

describe("auditActionLabel", () => {
    it("names a known action", () => {
        // given
        const action = "set_user_email";

        // when
        const label = auditActionLabel(action);

        // then
        expect(label).toBe("Email change started");
    });

    it("shows an action this page does not know as the server sent it", () => {
        // given
        const action = "rotate_keys";

        // when
        const label = auditActionLabel(action);

        // then
        expect(label).toBe("rotate_keys");
    });
});

describe("auditDetailLabel", () => {
    it("turns a detail key into sentence case words", () => {
        // given
        const key = "previous_display_name";

        // when
        const label = auditDetailLabel(key);

        // then
        expect(label).toBe("Previous display name");
    });

    it("calls the provider detail what it holds, the provider's id", () => {
        // given
        const key = "provider";

        // when
        const label = auditDetailLabel(key);

        // then
        expect(label).toBe("Provider ID");
    });

    it("keeps a key with no words as it is", () => {
        // given
        const key = "__";

        // when
        const label = auditDetailLabel(key);

        // then
        expect(label).toBe("__");
    });
});

describe("auditDetails", () => {
    it("lists the details in key order with readable labels", () => {
        // given
        const entry = makeAuditLogEntry({
            target_type: "user",
            details: { role: "admin", previous_role: "support" },
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([
            { key: "previous_role", label: "Previous role", value: "support" },
            { key: "role", label: "Role", value: "admin" },
        ]);
    });

    it("names a target that is not a user first", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "update_role_permissions",
            target_type: "role",
            target_id: "support",
            details: { permissions: "ban_user,view_users" },
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([
            { key: "target", label: "Role", value: "support" },
            { key: "permissions", label: "Permissions", value: "ban_user,view_users" },
        ]);
    });

    it("names a sign-in method by its provider's name, and keeps the provider's id beside it", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "identity_linked",
            target_type: "identity",
            target_id: "google",
            details: { provider: "google", provider_name: "Google", method: "automatic" },
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([
            { key: "target", label: "Sign-in method", value: "Google" },
            { key: "method", label: "Method", value: "automatic" },
            { key: "provider", label: "Provider ID", value: "google" },
        ]);
    });

    it("names each provider's sign-in method by that provider's name", () => {
        // given
        const entries = [
            makeAuditLogEntry({
                action: "identity_unlinked",
                target_type: "identity",
                target_id: "google",
                details: { provider: "google", provider_name: "Google" },
            }),
            makeAuditLogEntry({
                action: "identity_unlinked",
                target_type: "identity",
                target_id: "example",
                details: { provider: "example", provider_name: "Example ID" },
            }),
        ];

        // when
        const targets = entries.map(entry => auditDetails(entry)[0]);

        // then
        expect(targets).toEqual([
            { key: "target", label: "Sign-in method", value: "Google" },
            { key: "target", label: "Sign-in method", value: "Example ID" },
        ]);
    });

    it("names a sign-in method by its provider's id when the entry carries no name", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "identity_unlinked",
            target_type: "identity",
            target_id: "google",
            details: { provider: "google" },
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([
            { key: "target", label: "Sign-in method", value: "google" },
            { key: "provider", label: "Provider ID", value: "google" },
        ]);
    });

    it("names an invitation by its id first, then the address it went to", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "invite_user",
            target_type: "invitation",
            target_id: "00000000-0000-4000-8000-0000000000bb",
            details: { email: "kit@example.test" },
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([
            { key: "target", label: "Invitation", value: "00000000-0000-4000-8000-0000000000bb" },
            { key: "email", label: "Email", value: "kit@example.test" },
        ]);
    });

    it("lists a provider name on an entry about anything else as an ordinary detail", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "update_settings",
            target_type: "settings",
            target_id: undefined,
            details: { provider_name: "Google" },
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([{ key: "provider_name", label: "Provider name", value: "Google" }]);
    });

    it("leaves out a target with no id", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "update_settings",
            target_type: "settings",
            target_id: undefined,
            details: {},
        });

        // when
        const details = auditDetails(entry);

        // then
        expect(details).toEqual([]);
    });
});

describe("auditActorName", () => {
    it("names the actor when the entry carries a name", () => {
        // given
        const entry = makeAuditLogEntry({ actor_name: "Sam" });

        // when
        const name = auditActorName(entry);

        // then
        expect(name).toBe("Sam");
    });

    it("names the system for a system entry", () => {
        // given
        const entry = makeAuditLogEntry({ actor_kind: "system", actor_id: undefined, actor_name: undefined });

        // when
        const name = auditActorName(entry);

        // then
        expect(name).toBe("System");
    });

    it("says the account is gone when a person's name is missing", () => {
        // given
        const entry = makeAuditLogEntry({ actor_kind: "staff", actor_name: "" });

        // when
        const name = auditActorName(entry);

        // then
        expect(name).toBe("Deleted account");
    });
});
