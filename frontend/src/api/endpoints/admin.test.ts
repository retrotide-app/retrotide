import { beforeEach, describe, vi } from "vitest";
import * as api from "./admin";
import {
    deleteMock,
    deleteWithBodyMock,
    fetchMock,
    postMock,
    putMock,
    resetTransports,
    runRequestCases,
    type RequestCase,
} from "./testHarness";

vi.mock("../client", async importOriginal => {
    const actual = await importOriginal<typeof import("../client")>();

    return {
        ...actual,
        apiFetch: vi.fn(),
        apiPost: vi.fn(),
        apiPut: vi.fn(),
        apiPatch: vi.fn(),
        apiDelete: vi.fn(),
        apiDeleteWithBody: vi.fn(),
    };
});

beforeEach(resetTransports);

const USER_ID = "00000000-0000-4000-8000-0000000000aa";

describe("user records", () => {
    const cases: RequestCase[] = [
        {
            name: "getAdminUsers sends the search and the page",
            call: () => api.getAdminUsers({ search: "sam", limit: 20, offset: 40 }),
            transport: fetchMock,
            request: ["/admin/users?search=sam&limit=20&offset=40"],
            strict: true,
        },
        {
            name: "getAdminUsers leaves out a blank search and the first page's offset",
            call: () => api.getAdminUsers({ search: "", limit: 20, offset: 0 }),
            transport: fetchMock,
            request: ["/admin/users?limit=20"],
            strict: true,
        },
        {
            name: "getAdminUsers asks for the server's defaults when given nothing",
            call: () => api.getAdminUsers({}),
            transport: fetchMock,
            request: ["/admin/users"],
            strict: true,
        },
        {
            name: "getAdminUsers escapes the search",
            call: () => api.getAdminUsers({ search: "sam&robin" }),
            transport: fetchMock,
            request: ["/admin/users?search=sam%26robin"],
            strict: true,
        },
        {
            name: "getAdminUser reads one user's record",
            call: () => api.getAdminUser(USER_ID),
            transport: fetchMock,
            request: [`/admin/users/${USER_ID}`],
            strict: true,
        },
        {
            name: "getAdminUserAuditLog reads a page of one user's audit log",
            call: () => api.getAdminUserAuditLog(USER_ID, { limit: 50, offset: 50 }),
            transport: fetchMock,
            request: [`/admin/users/${USER_ID}/audit-log?limit=50&offset=50`],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("user actions", () => {
    const cases: RequestCase[] = [
        {
            name: "setUserRole posts the role",
            call: () => api.setUserRole(USER_ID, { role: "support" }),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/role`, { role: "support" }],
            strict: true,
        },
        {
            name: "removeUserRole deletes with the role in the body",
            call: () => api.removeUserRole(USER_ID, { role: "support" }),
            transport: deleteWithBodyMock,
            request: [`/admin/users/${USER_ID}/role`, { role: "support" }],
            strict: true,
        },
        {
            name: "banUser posts the reason",
            call: () => api.banUser(USER_ID, { reason: "spam" }),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/ban`, { reason: "spam" }],
            strict: true,
        },
        {
            name: "unbanUser posts with no body",
            call: () => api.unbanUser(USER_ID),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/unban`],
            strict: true,
        },
        {
            name: "lockUser posts the reason",
            call: () => api.lockUser(USER_ID, { reason: "chargeback" }),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/lock`, { reason: "chargeback" }],
            strict: true,
        },
        {
            name: "unlockUser posts with no body",
            call: () => api.unlockUser(USER_ID),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/unlock`],
            strict: true,
        },
        {
            name: "forceLogoutUser posts with no body",
            call: () => api.forceLogoutUser(USER_ID),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/force-logout`],
            strict: true,
        },
        {
            name: "deleteUser deletes the user",
            call: () => api.deleteUser(USER_ID),
            transport: deleteMock,
            request: [`/admin/users/${USER_ID}`],
            strict: true,
        },
        {
            name: "resetUserPassword posts with no body",
            call: () => api.resetUserPassword(USER_ID),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/reset-password`],
            strict: true,
        },
        {
            name: "setUserEmail puts the new address",
            call: () => api.setUserEmail(USER_ID, { email: "robin@example.test" }),
            transport: putMock,
            request: [`/admin/users/${USER_ID}/email`, { email: "robin@example.test" }],
            strict: true,
        },
        {
            name: "verifyUserEmail posts with no body",
            call: () => api.verifyUserEmail(USER_ID),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/verify-email`],
            strict: true,
        },
        {
            name: "unverifyUserEmail posts with no body",
            call: () => api.unverifyUserEmail(USER_ID),
            transport: postMock,
            request: [`/admin/users/${USER_ID}/unverify-email`],
            strict: true,
        },
        {
            name: "setUserDisplayName puts the display name",
            call: () => api.setUserDisplayName(USER_ID, { display_name: "Robin" }),
            transport: putMock,
            request: [`/admin/users/${USER_ID}/display-name`, { display_name: "Robin" }],
            strict: true,
        },
        {
            name: "a user id is escaped before it reaches the path",
            call: () => api.getAdminUser("a/b"),
            transport: fetchMock,
            request: ["/admin/users/a%2Fb"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("invitations", () => {
    const INVITATION_ID = "00000000-0000-4000-8000-0000000000bb";

    const cases: RequestCase[] = [
        {
            name: "getAdminInvitations reads a page of the pending invitations",
            call: () => api.getAdminInvitations({ limit: 20, offset: 40 }),
            transport: fetchMock,
            request: ["/admin/invitations?limit=20&offset=40"],
            strict: true,
        },
        {
            name: "getAdminInvitations leaves out the first page's offset",
            call: () => api.getAdminInvitations({ limit: 20, offset: 0 }),
            transport: fetchMock,
            request: ["/admin/invitations?limit=20"],
            strict: true,
        },
        {
            name: "inviteUser posts the address",
            call: () => api.inviteUser({ email: "robin@example.test" }),
            transport: postMock,
            request: ["/admin/invitations", { email: "robin@example.test" }],
            strict: true,
        },
        {
            name: "resendInvitation posts with no body",
            call: () => api.resendInvitation(INVITATION_ID),
            transport: postMock,
            request: [`/admin/invitations/${INVITATION_ID}/resend`],
            strict: true,
        },
        {
            name: "cancelInvitation deletes the invitation",
            call: () => api.cancelInvitation(INVITATION_ID),
            transport: deleteMock,
            request: [`/admin/invitations/${INVITATION_ID}`],
            strict: true,
        },
        {
            name: "an invitation id is escaped before it reaches the path",
            call: () => api.cancelInvitation("a/b"),
            transport: deleteMock,
            request: ["/admin/invitations/a%2Fb"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("settings", () => {
    const cases: RequestCase[] = [
        {
            name: "getAdminSettings reads every setting",
            call: () => api.getAdminSettings(),
            transport: fetchMock,
            request: ["/admin/settings"],
            strict: true,
        },
        {
            name: "updateAdminSettings puts the changed settings",
            call: () => api.updateAdminSettings({ settings: { site_mode: "private" } }),
            transport: putMock,
            request: ["/admin/settings", { settings: { site_mode: "private" } }],
            strict: true,
        },
        {
            name: "sendTestEmail posts with no body",
            call: () => api.sendTestEmail(),
            transport: postMock,
            request: ["/admin/settings/test-email"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("permissions", () => {
    const cases: RequestCase[] = [
        {
            name: "getAdminPermissions reads the catalogue and the editable roles",
            call: () => api.getAdminPermissions(),
            transport: fetchMock,
            request: ["/admin/permissions"],
            strict: true,
        },
        {
            name: "updateRolePermissions puts the role's permissions",
            call: () => api.updateRolePermissions("support", { permissions: ["view_users", "ban_user"] }),
            transport: putMock,
            request: ["/admin/permissions/roles/support", { permissions: ["view_users", "ban_user"] }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("audit log", () => {
    const cases: RequestCase[] = [
        {
            name: "getAuditLog filters by action and pages",
            call: () => api.getAuditLog({ action: "ban_user", limit: 50, offset: 50 }),
            transport: fetchMock,
            request: ["/admin/audit-log?action=ban_user&limit=50&offset=50"],
            strict: true,
        },
        {
            name: "getAuditLog reads every action when no filter is set",
            call: () => api.getAuditLog({ limit: 50 }),
            transport: fetchMock,
            request: ["/admin/audit-log?limit=50"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
