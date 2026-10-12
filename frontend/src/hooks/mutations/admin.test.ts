import type { QueryKey } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useBanUser,
    useCancelInvitation,
    useDeleteUser,
    useForceLogoutUser,
    useInviteUser,
    useLockUser,
    useRemoveUserRole,
    useResendInvitation,
    useResetUserPassword,
    useSendTestEmail,
    useSetUserDisplayName,
    useSetUserEmail,
    useSetUserRole,
    useUnbanUser,
    useUnlockUser,
    useUnverifyUserEmail,
    useUpdateAdminSettings,
    useUpdateRolePermissions,
    useVerifyUserEmail,
} from "./admin";

const endpoints = vi.hoisted(() => ({
    banUser: vi.fn(),
    cancelInvitation: vi.fn(),
    deleteUser: vi.fn(),
    forceLogoutUser: vi.fn(),
    inviteUser: vi.fn(),
    lockUser: vi.fn(),
    removeUserRole: vi.fn(),
    resendInvitation: vi.fn(),
    resetUserPassword: vi.fn(),
    sendTestEmail: vi.fn(),
    setUserDisplayName: vi.fn(),
    setUserEmail: vi.fn(),
    setUserRole: vi.fn(),
    unbanUser: vi.fn(),
    unlockUser: vi.fn(),
    unverifyUserEmail: vi.fn(),
    updateAdminSettings: vi.fn(),
    updateRolePermissions: vi.fn(),
    verifyUserEmail: vi.fn(),
}));

vi.mock("../../api/endpoints/admin", () => endpoints);

interface MutationCase {
    name: string;
    use: () => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
    invalidated: QueryKey[];
}

const USER_RECORDS: QueryKey[] = [
    queryKeys.admin.users.all,
    queryKeys.admin.organisations.all,
    queryKeys.admin.stats(),
    queryKeys.admin.auditLog.all,
];

const INVITATION_RECORDS: QueryKey[] = [queryKeys.admin.invitations.all, queryKeys.admin.auditLog.all];

const cases: MutationCase[] = [
    {
        name: "useSetUserRole grants the role and refreshes the user records",
        use: useSetUserRole,
        variables: { userId: "u1", role: "support" },
        endpoint: endpoints.setUserRole,
        request: ["u1", { role: "support" }],
        invalidated: USER_RECORDS,
    },
    {
        name: "useRemoveUserRole takes the role away and refreshes the user records",
        use: useRemoveUserRole,
        variables: { userId: "u1", role: "support" },
        endpoint: endpoints.removeUserRole,
        request: ["u1", { role: "support" }],
        invalidated: USER_RECORDS,
    },
    {
        name: "useBanUser bans with the reason and refreshes the user records",
        use: useBanUser,
        variables: { userId: "u1", reason: "spam" },
        endpoint: endpoints.banUser,
        request: ["u1", { reason: "spam" }],
        invalidated: USER_RECORDS,
    },
    {
        name: "useUnbanUser lifts the ban and refreshes the user records",
        use: useUnbanUser,
        variables: { userId: "u1" },
        endpoint: endpoints.unbanUser,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useLockUser locks with the reason and refreshes the user records",
        use: useLockUser,
        variables: { userId: "u1", reason: "chargeback" },
        endpoint: endpoints.lockUser,
        request: ["u1", { reason: "chargeback" }],
        invalidated: USER_RECORDS,
    },
    {
        name: "useUnlockUser unlocks and refreshes the user records",
        use: useUnlockUser,
        variables: { userId: "u1" },
        endpoint: endpoints.unlockUser,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useForceLogoutUser signs the user out everywhere and refreshes the user records",
        use: useForceLogoutUser,
        variables: { userId: "u1" },
        endpoint: endpoints.forceLogoutUser,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useDeleteUser deletes the account and refreshes the user records",
        use: useDeleteUser,
        variables: { userId: "u1" },
        endpoint: endpoints.deleteUser,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useResetUserPassword resets the password and refreshes the user records",
        use: useResetUserPassword,
        variables: { userId: "u1" },
        endpoint: endpoints.resetUserPassword,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useSetUserEmail starts the change of address and refreshes the user records",
        use: useSetUserEmail,
        variables: { userId: "u1", email: "robin@example.test" },
        endpoint: endpoints.setUserEmail,
        request: ["u1", { email: "robin@example.test" }],
        invalidated: USER_RECORDS,
    },
    {
        name: "useVerifyUserEmail marks the address verified and refreshes the user records",
        use: useVerifyUserEmail,
        variables: { userId: "u1" },
        endpoint: endpoints.verifyUserEmail,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useUnverifyUserEmail marks the address unverified and refreshes the user records",
        use: useUnverifyUserEmail,
        variables: { userId: "u1" },
        endpoint: endpoints.unverifyUserEmail,
        request: ["u1"],
        invalidated: USER_RECORDS,
    },
    {
        name: "useSetUserDisplayName renames the user and refreshes the user records",
        use: useSetUserDisplayName,
        variables: { userId: "u1", display_name: "Robin" },
        endpoint: endpoints.setUserDisplayName,
        request: ["u1", { display_name: "Robin" }],
        invalidated: USER_RECORDS,
    },
    {
        name: "useInviteUser invites the address and refreshes the invitations",
        use: useInviteUser,
        variables: { email: "kit@example.test" },
        endpoint: endpoints.inviteUser,
        request: [{ email: "kit@example.test" }],
        invalidated: INVITATION_RECORDS,
    },
    {
        name: "useResendInvitation sends the invitation again and refreshes the invitations",
        use: useResendInvitation,
        variables: { invitationId: "i1" },
        endpoint: endpoints.resendInvitation,
        request: ["i1"],
        invalidated: INVITATION_RECORDS,
    },
    {
        name: "useCancelInvitation cancels the invitation and refreshes the invitations",
        use: useCancelInvitation,
        variables: { invitationId: "i1" },
        endpoint: endpoints.cancelInvitation,
        request: ["i1"],
        invalidated: INVITATION_RECORDS,
    },
    {
        name: "useUpdateAdminSettings saves the settings and refreshes them, the site info and the audit log",
        use: useUpdateAdminSettings,
        variables: { settings: { site_mode: "private" } },
        endpoint: endpoints.updateAdminSettings,
        request: [{ settings: { site_mode: "private" } }],
        invalidated: [
            queryKeys.admin.settings(),
            queryKeys.siteInfo(),
            queryKeys.admin.stats(),
            queryKeys.admin.auditLog.all,
        ],
    },
    {
        name: "useSendTestEmail sends the test email and refreshes the audit log",
        use: useSendTestEmail,
        variables: undefined,
        endpoint: endpoints.sendTestEmail,
        request: [undefined],
        invalidated: [queryKeys.admin.auditLog.all],
    },
    {
        name: "useUpdateRolePermissions saves the role's permissions and refreshes them and the audit log",
        use: useUpdateRolePermissions,
        variables: { role: "support", permissions: ["view_users"] },
        endpoint: endpoints.updateRolePermissions,
        request: ["support", { permissions: ["view_users"] }],
        invalidated: [queryKeys.admin.permissions(), queryKeys.admin.auditLog.all],
    },
];

const NOTHING_SAVED_ON_FAILURE = new Set<MutationCase["use"]>([useSendTestEmail]);

const savedDespiteServerFailure = cases.filter(({ use }) => !NOTHING_SAVED_ON_FAILURE.has(use));

const STALE_CACHE_MESSAGE =
    "the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply";

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockResolvedValue(undefined);
    }
});

describe("admin mutations", () => {
    it.each(cases)("$name", async ({ use, variables, endpoint, request, invalidated }) => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
        });

        // then
        expect(endpoint.mock.calls[0].slice(0, request.length)).toEqual(request);
        for (const key of invalidated) {
            expectInvalidated(qc, key);
        }
        expect(invalidate).toHaveBeenCalledTimes(invalidated.length);
    });

    it.each(cases)("leaves every cache alone when the server refuses: $name", async ({ use, variables, endpoint }) => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoint.mockRejectedValue(new Error("insufficient permissions"));
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(variables as never)).rejects.toThrow("insufficient permissions");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it.each(savedDespiteServerFailure)(
        "refreshes after a 500, which the server also answers for a saved change: $name",
        async ({ use, variables, endpoint, invalidated }) => {
            // given
            const qc = createTestQueryClient();
            const invalidate = vi.spyOn(qc, "invalidateQueries");
            endpoint.mockRejectedValue(makeApiError({ status: 500, message: STALE_CACHE_MESSAGE }));
            const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

            // when
            await act(async () => {
                await expect(result.current.mutateAsync(variables as never)).rejects.toThrow(STALE_CACHE_MESSAGE);
            });

            // then
            for (const key of invalidated) {
                expectInvalidated(qc, key);
            }
            expect(invalidate).toHaveBeenCalledTimes(invalidated.length);
        },
    );

    it.each([
        { name: "useResendInvitation", use: useResendInvitation, endpoint: endpoints.resendInvitation },
        { name: "useCancelInvitation", use: useCancelInvitation, endpoint: endpoints.cancelInvitation },
    ])("refreshes the invitations after a 404, because the invitation is gone: $name", async ({ use, endpoint }) => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoint.mockRejectedValue(makeApiError({ status: 404, message: "invitation not found" }));
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ invitationId: "i1" })).rejects.toThrow("invitation not found");
        });

        // then
        for (const key of INVITATION_RECORDS) {
            expectInvalidated(qc, key);
        }
        expect(invalidate).toHaveBeenCalledTimes(INVITATION_RECORDS.length);
    });

    it("leaves the invitations alone when the server refuses an address", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoints.inviteUser.mockRejectedValue(
            makeApiError({
                status: 409,
                message: "that email address already has an account",
                code: "account_exists",
            }),
        );
        const { result } = renderHook(useInviteUser, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ email: "kit@example.test" })).rejects.toThrow(
                "that email address already has an account",
            );
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("drops the deleted user's record and audit log instead of fetching them again", async () => {
        // given
        const qc = createTestQueryClient();
        const remove = vi.spyOn(qc, "removeQueries");
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const { result } = renderHook(useDeleteUser, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync({ userId: "u1" });
        });

        // then
        expect(remove).toHaveBeenCalledWith({ queryKey: queryKeys.admin.users.detail("u1") });
        expect(remove).toHaveBeenCalledWith({ queryKey: queryKeys.admin.users.auditLogPages("u1") });
        expect(remove.mock.invocationCallOrder[1]).toBeLessThan(invalidate.mock.invocationCallOrder[0]);
    });

    it("hands back the generated password once", async () => {
        // given
        const qc = createTestQueryClient();
        endpoints.resetUserPassword.mockResolvedValue({ password: "tide-harbour-lamp" });
        const { result } = renderHook(useResetUserPassword, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let reset: unknown;
        await act(async () => {
            reset = await result.current.mutateAsync({ userId: "u1" });
        });

        // then
        expect(reset).toEqual({ password: "tide-harbour-lamp" });
    });
});
