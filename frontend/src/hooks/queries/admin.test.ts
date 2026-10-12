import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import {
    makeAdminInvitation,
    makeAdminInvitationList,
    makeAdminPermissions,
    makeAdminUser,
    makeAdminUserDetail,
    makeAdminUserList,
    makeApiError,
    makeAuditLogEntry,
    makeAuditLogList,
} from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useAdminInvitations,
    useAdminPermissions,
    useAdminSettings,
    useAdminUser,
    useAdminUserAuditLog,
    useAdminUsers,
    useAuditLog,
} from "./admin";

const endpoints = vi.hoisted(() => ({
    getAdminInvitations: vi.fn(),
    getAdminUsers: vi.fn(),
    getAdminUser: vi.fn(),
    getAdminUserAuditLog: vi.fn(),
    getAdminSettings: vi.fn(),
    getAdminPermissions: vi.fn(),
    getAuditLog: vi.fn(),
}));

vi.mock("../../api/endpoints/admin", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getAdminInvitations.mockResolvedValue(makeAdminInvitationList([]));
    endpoints.getAdminUsers.mockResolvedValue(makeAdminUserList([]));
    endpoints.getAdminUser.mockResolvedValue(makeAdminUserDetail());
    endpoints.getAdminUserAuditLog.mockResolvedValue(makeAuditLogList([]));
    endpoints.getAdminSettings.mockResolvedValue({ settings: {} });
    endpoints.getAdminPermissions.mockResolvedValue(makeAdminPermissions());
    endpoints.getAuditLog.mockResolvedValue(makeAuditLogList([]));
});

describe("useAdminUsers", () => {
    it("reads a page of users under the key for its search and page", async () => {
        // given
        const query = { search: "sam", limit: 20, offset: 0 };
        const listing = makeAdminUserList([makeAdminUser()]);
        endpoints.getAdminUsers.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useAdminUsers(query));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.getAdminUsers).toHaveBeenCalledWith(query);
        expect(queryClient.getQueryData(queryKeys.admin.users.list(query))).toEqual(listing);
    });

    it("keeps the last page on show while the next one loads", async () => {
        // given
        const first = makeAdminUserList([makeAdminUser({ display_name: "Sam" })]);
        endpoints.getAdminUsers.mockResolvedValueOnce(first).mockReturnValueOnce(new Promise(() => {}));
        const { result, rerender } = renderHook(({ offset }) => useAdminUsers({ offset }), {
            wrapper: providerWrapper(),
            initialProps: { offset: 0 },
        });
        await waitFor(() => expect(result.current.data).toEqual(first));

        // when
        rerender({ offset: 20 });

        // then
        expect(result.current.data).toEqual(first);
        expect(result.current.loading).toBe(false);
    });

    it("hands back the error when the users could not be listed", async () => {
        // given
        const failure = makeApiError({ status: 403, message: "insufficient permissions" });
        endpoints.getAdminUsers.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useAdminUsers({}));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});

describe("useAdminUser", () => {
    it("reads one user's record under that user's key", async () => {
        // given
        const detail = makeAdminUserDetail({ id: "u1" });
        endpoints.getAdminUser.mockResolvedValue(detail);

        // when
        const { result, queryClient } = setup(() => useAdminUser("u1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(detail));
        expect(endpoints.getAdminUser).toHaveBeenCalledWith("u1");
        expect(queryClient.getQueryData(queryKeys.admin.users.detail("u1"))).toEqual(detail);
    });

    it("asks nothing without a user id", () => {
        // when
        const { result } = setup(() => useAdminUser(""));

        // then
        expect(endpoints.getAdminUser).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});

describe("useAdminUserAuditLog", () => {
    it("reads a page of the user's audit log under that user's key", async () => {
        // given
        const listing = makeAuditLogList([makeAuditLogEntry()]);
        endpoints.getAdminUserAuditLog.mockResolvedValue(listing);
        const page = { limit: 50, offset: 0 };

        // when
        const { result, queryClient } = setup(() => useAdminUserAuditLog("u1", page));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.getAdminUserAuditLog).toHaveBeenCalledWith("u1", page);
        expect(queryClient.getQueryData(queryKeys.admin.users.auditLog("u1", page))).toEqual(listing);
    });

    it("asks nothing while it is switched off, as for staff without the audit log permission", () => {
        // when
        const { result } = setup(() => useAdminUserAuditLog("u1", {}, false));

        // then
        expect(endpoints.getAdminUserAuditLog).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});

describe("useAdminInvitations", () => {
    it("reads a page of the pending invitations under the key for its page", async () => {
        // given
        const page = { limit: 20, offset: 0 };
        const listing = makeAdminInvitationList([makeAdminInvitation()]);
        endpoints.getAdminInvitations.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useAdminInvitations(page));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.getAdminInvitations).toHaveBeenCalledWith(page);
        expect(queryClient.getQueryData(queryKeys.admin.invitations.list(page))).toEqual(listing);
    });

    it("keeps the last page on show while the next one loads", async () => {
        // given
        const first = makeAdminInvitationList([makeAdminInvitation()]);
        endpoints.getAdminInvitations.mockResolvedValueOnce(first).mockReturnValueOnce(new Promise(() => {}));
        const { result, rerender } = renderHook(({ offset }) => useAdminInvitations({ offset }), {
            wrapper: providerWrapper(),
            initialProps: { offset: 0 },
        });
        await waitFor(() => expect(result.current.data).toEqual(first));

        // when
        rerender({ offset: 20 });

        // then
        expect(result.current.data).toEqual(first);
        expect(result.current.loading).toBe(false);
    });

    it("hands back the error when the invitations could not be listed", async () => {
        // given
        const failure = makeApiError({ status: 403, message: "insufficient permissions" });
        endpoints.getAdminInvitations.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useAdminInvitations({}));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});

describe("useAdminSettings", () => {
    it("reads the settings under the settings key", async () => {
        // given
        const settings = { settings: { site_name: "Harbour", turnstile_secret_key: "********" } };
        endpoints.getAdminSettings.mockResolvedValue(settings);

        // when
        const { result, queryClient } = setup(useAdminSettings);

        // then
        await waitFor(() => expect(result.current.data).toEqual(settings));
        expect(queryClient.getQueryData(queryKeys.admin.settings())).toEqual(settings);
    });
});

describe("useAdminPermissions", () => {
    it("reads the catalogue and roles under the permissions key", async () => {
        // given
        const permissions = makeAdminPermissions();
        endpoints.getAdminPermissions.mockResolvedValue(permissions);

        // when
        const { result, queryClient } = setup(useAdminPermissions);

        // then
        await waitFor(() => expect(result.current.data).toEqual(permissions));
        expect(queryClient.getQueryData(queryKeys.admin.permissions())).toEqual(permissions);
    });
});

describe("useAuditLog", () => {
    it("reads a filtered page of the site audit log under the key for its filter", async () => {
        // given
        const listing = makeAuditLogList([makeAuditLogEntry({ action: "ban_user" })]);
        endpoints.getAuditLog.mockResolvedValue(listing);
        const query = { action: "ban_user" as const, limit: 50, offset: 0 };

        // when
        const { result, queryClient } = setup(() => useAuditLog(query));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.getAuditLog).toHaveBeenCalledWith(query);
        expect(queryClient.getQueryData(queryKeys.admin.auditLog.list(query))).toEqual(listing);
    });
});
