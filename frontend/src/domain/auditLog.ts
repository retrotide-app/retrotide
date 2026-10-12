import type { AuditAction, AuditLogEntryResponse, AuditTargetType } from "../types/api";

export interface AuditDetail {
    key: string;
    label: string;
    value: string;
}

export const AUDIT_ACTION_LABELS: Readonly<Record<AuditAction, string>> = {
    org_created: "Organisation created",
    org_updated: "Organisation updated",
    org_deleted: "Organisation deleted",
    org_restored: "Organisation restored",
    org_ownership_transferred: "Organisation ownership transferred",
    member_invited: "Member invited",
    invite_revoked: "Organisation invite revoked",
    member_joined: "Member joined",
    member_role_changed: "Member role changed",
    member_removed: "Member removed",
    team_created: "Team created",
    team_archived: "Team archived",
    team_deleted: "Team deleted",
    team_member_added: "Team member added",
    team_member_role_changed: "Team member role changed",
    team_member_removed: "Team member removed",
    board_created: "Board created",
    board_archived: "Board archived",
    board_deleted: "Board deleted",
    board_restored: "Board restored",
    board_sharing_changed: "Board sharing changed",
    board_password_changed: "Board password changed",
    board_link_rotated: "Board link rotated",
    board_closed: "Board closed",
    subscription_changed: "Subscription changed",
    entitlement_override_set: "Entitlement override set",
    guest_claimed: "Guest claimed",
    identity_linked: "Sign-in method linked",
    identity_unlinked: "Sign-in method removed",
    invitation_accepted: "Invitation accepted",
    two_factor_enabled: "Two-factor authentication turned on",
    two_factor_disabled: "Two-factor authentication turned off",
    set_role: "Role set",
    remove_role: "Role removed",
    ban_user: "User banned",
    unban_user: "User unbanned",
    lock_user: "User locked",
    unlock_user: "User unlocked",
    delete_user: "User deleted",
    reset_password: "Password reset",
    set_user_email: "Email change started",
    verify_user_email: "Email marked verified",
    unverify_user_email: "Email marked unverified",
    set_display_name: "Display name changed",
    force_logout: "Signed out everywhere",
    update_settings: "Settings updated",
    send_test_email: "Test email sent",
    update_role_permissions: "Role permissions updated",
    invite_user: "User invited",
    resend_invitation: "Invitation sent again",
    cancel_invitation: "Invitation cancelled",
};

export const SITE_AUDIT_ACTIONS: readonly AuditAction[] = [
    "ban_user",
    "unban_user",
    "lock_user",
    "unlock_user",
    "force_logout",
    "delete_user",
    "reset_password",
    "set_user_email",
    "verify_user_email",
    "unverify_user_email",
    "set_display_name",
    "set_role",
    "remove_role",
    "update_role_permissions",
    "update_settings",
    "send_test_email",
    "invite_user",
    "resend_invitation",
    "cancel_invitation",
    "invitation_accepted",
    "identity_linked",
    "identity_unlinked",
    "two_factor_enabled",
    "two_factor_disabled",
];

export const AUDIT_TARGET_LABELS: Readonly<Record<AuditTargetType, string>> = {
    user: "User",
    settings: "Settings",
    role: "Role",
    identity: "Sign-in method",
    invitation: "Invitation",
};

const AUDIT_DETAIL_LABELS: Readonly<Record<string, string>> = {
    provider: "Provider ID",
};

const PROVIDER_NAME_DETAIL = "provider_name";

export function isAuditAction(value: string): value is AuditAction {
    return Object.hasOwn(AUDIT_ACTION_LABELS, value);
}

export function auditActionLabel(action: string): string {
    if (isAuditAction(action)) {
        return AUDIT_ACTION_LABELS[action];
    }

    return action;
}

export function auditDetailLabel(key: string): string {
    if (Object.hasOwn(AUDIT_DETAIL_LABELS, key)) {
        return AUDIT_DETAIL_LABELS[key];
    }

    const words = key.split("_").filter(word => word !== "");

    if (words.length === 0) {
        return key;
    }

    const sentence = words.join(" ");

    return sentence.charAt(0).toUpperCase() + sentence.slice(1);
}

function auditTargetName(entry: AuditLogEntryResponse): string {
    const providerName = entry.details[PROVIDER_NAME_DETAIL];

    if (entry.target_type === "identity" && providerName !== undefined && providerName !== "") {
        return providerName;
    }

    return entry.target_id ?? "";
}

export function auditDetails(entry: AuditLogEntryResponse): AuditDetail[] {
    const details: AuditDetail[] = [];
    const target = auditTargetName(entry);

    if (entry.target_type !== "user" && target !== "") {
        details.push({ key: "target", label: AUDIT_TARGET_LABELS[entry.target_type], value: target });
    }

    const namedByTarget = entry.target_type === "identity" ? PROVIDER_NAME_DETAIL : null;
    const keys = Object.keys(entry.details)
        .filter(key => key !== namedByTarget)
        .sort();

    for (const key of keys) {
        details.push({ key, label: auditDetailLabel(key), value: entry.details[key] });
    }

    return details;
}

export function auditActorName(entry: AuditLogEntryResponse): string {
    if (entry.actor_name !== undefined && entry.actor_name !== "") {
        return entry.actor_name;
    }

    if (entry.actor_kind === "system") {
        return "System";
    }

    return "Deleted account";
}
