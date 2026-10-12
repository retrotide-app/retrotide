import type { QueryClient } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { clearSessionLost, isSessionLost, markSessionLost } from "../../api/sessionLost";
import { makeSession, makeSessionUser, makeSiteInfo } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useAcceptInvitation,
    useChangePassword,
    useDeleteAccount,
    useForgotPassword,
    useLogin,
    useLogout,
    useRegister,
    useResendVerification,
    useResetPassword,
    useSetEmail,
    useSignOutOtherDevices,
    useUpdateProfile,
    useVerifyEmail,
} from "./auth";

const endpoints = vi.hoisted(() => ({
    acceptInvitation: vi.fn(),
    changePassword: vi.fn(),
    deleteAccount: vi.fn(),
    forgotPassword: vi.fn(),
    getSession: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
    register: vi.fn(),
    resendVerification: vi.fn(),
    resetPassword: vi.fn(),
    setEmail: vi.fn(),
    signOutOtherDevices: vi.fn(),
    updateProfile: vi.fn(),
    verifyEmail: vi.fn(),
}));

vi.mock("../../api/endpoints/auth", () => endpoints);

function client() {
    const qc = createTestQueryClient();
    const defaults = qc.getDefaultOptions();
    qc.setDefaultOptions({ ...defaults, queries: { ...defaults.queries, gcTime: Infinity } });

    return { qc, invalidate: vi.spyOn(qc, "invalidateQueries") };
}

async function runMutation<V>(
    use: () => { mutateAsync: (variables: V) => Promise<unknown> },
    variables: V,
    qc: QueryClient,
): Promise<unknown> {
    const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

    let data: unknown;
    await act(async () => {
        data = await result.current.mutateAsync(variables);
    });

    return data;
}

function seedAccountData(qc: QueryClient): void {
    qc.setQueryData(queryKeys.siteInfo(), makeSiteInfo());
    qc.setQueryData(queryKeys.auth.session(), makeSession());
    qc.setQueryData(queryKeys.auth.identities(), { identities: [] });
    qc.setQueryData(queryKeys.admin.settings(), { settings: {} });
}

beforeEach(() => {
    clearSessionLost();

    for (const fn of Object.values(endpoints)) {
        fn.mockResolvedValue(undefined);
    }
});

describe("useRegister", () => {
    it("sends the form and hands back the answer, leaving the session alone because a new account is not signed in", async () => {
        // given
        const { qc, invalidate } = client();
        seedAccountData(qc);
        const body = {
            email: "sam@example.test",
            password: "harbour-lights",
            display_name: "Sam",
            turnstile_token: "turnstile-1",
        };
        const registration = { email: "sam@example.test", verification_required: true };
        endpoints.register.mockResolvedValue(registration);

        // when
        const data = await runMutation(useRegister, body, qc);

        // then the site info is read again, because a first account closes registration on a private site
        expect(endpoints.register).toHaveBeenCalledWith(body, expect.anything());
        expect(data).toEqual(registration);
        expectInvalidated(qc, queryKeys.siteInfo());
        expect(invalidate).toHaveBeenCalledTimes(1);
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual(makeSession());
        expect(qc.getQueryData(queryKeys.auth.identities())).toEqual({ identities: [] });
    });

    it("keeps a lost session's banner, because registering signs nobody in", async () => {
        // given
        const { qc } = client();
        markSessionLost();

        // when
        await runMutation(useRegister, { email: "a@b.test", password: "pw", display_name: "" }, qc);

        // then
        expect(isSessionLost()).toBe(true);
    });

    it("leaves the session alone when the server refuses the registration", async () => {
        // given
        const { qc, invalidate } = client();
        endpoints.register.mockRejectedValue(new Error("this site is private, ask an admin to invite you"));
        const { result } = renderHook(useRegister, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({ email: "a@b.test", password: "pw", display_name: "" }),
            ).rejects.toThrow("this site is private, ask an admin to invite you");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useAcceptInvitation", () => {
    it("sends the token, the name and the password, and leaves the session alone because nobody is signed in", async () => {
        // given
        const { qc, invalidate } = client();
        seedAccountData(qc);
        const body = { token: "invite-1", display_name: "Robin", password: "harbour-lights" };
        endpoints.acceptInvitation.mockResolvedValue({ email: "robin@example.test" });

        // when
        const data = await runMutation(useAcceptInvitation, body, qc);

        // then
        expect(endpoints.acceptInvitation).toHaveBeenCalledExactlyOnceWith(body, expect.anything());
        expect(data).toEqual({ email: "robin@example.test" });
        expect(invalidate).not.toHaveBeenCalled();
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual(makeSession());
    });

    it("keeps a lost session's banner, because accepting an invitation signs nobody in", async () => {
        // given
        const { qc } = client();
        markSessionLost();

        // when
        await runMutation(useAcceptInvitation, { token: "invite-1", display_name: "Robin", password: "pw" }, qc);

        // then
        expect(isSessionLost()).toBe(true);
    });
});

describe("useLogin", () => {
    beforeEach(() => {
        endpoints.login.mockResolvedValue(makeSessionUser());
    });

    it("leaves the session and the account data alone while a second factor is still owed", async () => {
        // given
        const { qc, invalidate } = client();
        seedAccountData(qc);
        markSessionLost();
        endpoints.login.mockResolvedValue({ two_factor_required: true });

        // when
        const data = await runMutation(
            useLogin,
            { email: "sam@example.test", password: "harbour-lights", remember: false },
            qc,
        );

        // then nobody is signed in yet, so nothing is forgotten or reloaded
        expect(data).toEqual({ two_factor_required: true });
        expect(invalidate).not.toHaveBeenCalled();
        expect(isSessionLost()).toBe(true);
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual(makeSession());
        expect(qc.getQueryData(queryKeys.auth.identities())).toEqual({ identities: [] });
    });

    it("drops the last account's data but keeps the site info, then reloads the session", async () => {
        // given
        const { qc } = client();
        seedAccountData(qc);

        // when
        await runMutation(useLogin, { email: "sam@example.test", password: "harbour-lights", remember: false }, qc);

        // then
        expect(endpoints.login).toHaveBeenCalledWith(
            { email: "sam@example.test", password: "harbour-lights", remember: false },
            expect.anything(),
        );
        expect(qc.getQueryData(queryKeys.siteInfo())).toEqual(makeSiteInfo());
        expect(qc.getQueryData(queryKeys.auth.identities())).toBeUndefined();
        expect(qc.getQueryData(queryKeys.admin.settings())).toBeUndefined();
        expectInvalidated(qc, queryKeys.auth.session());
    });

    it("clears a lost session so the banner does not follow the new sign-in", async () => {
        // given
        const { qc } = client();
        markSessionLost();

        // when
        await runMutation(useLogin, { email: "sam@example.test", password: "harbour-lights", remember: false }, qc);

        // then
        expect(isSessionLost()).toBe(false);
    });

    it("leaves the session alone when the server refuses an unverified account", async () => {
        // given
        const { qc, invalidate } = client();
        seedAccountData(qc);
        endpoints.login.mockRejectedValue(new Error("verify your email address before signing in"));
        const { result } = renderHook(useLogin, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({ email: "sam@example.test", password: "harbour-lights", remember: false }),
            ).rejects.toThrow("verify your email address before signing in");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual(makeSession());
        expect(qc.getQueryData(queryKeys.auth.identities())).toEqual({ identities: [] });
    });

    it("passes the remember me choice and the turnstile token through", async () => {
        // given
        const { qc } = client();

        // when
        await runMutation(
            useLogin,
            { email: "sam@example.test", password: "pw", remember: true, turnstile_token: "t-1" },
            qc,
        );

        // then
        expect(endpoints.login).toHaveBeenCalledWith(
            { email: "sam@example.test", password: "pw", remember: true, turnstile_token: "t-1" },
            expect.anything(),
        );
    });
});

describe("useLogout", () => {
    it("marks the session signed out and drops everything the account could see", async () => {
        // given
        const { qc } = client();
        seedAccountData(qc);

        // when
        await runMutation(useLogout, undefined, qc);

        // then
        expect(endpoints.logout).toHaveBeenCalledOnce();
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual({ authenticated: false });
        expect(qc.getQueryData(queryKeys.siteInfo())).toEqual(makeSiteInfo());
        expect(qc.getQueryData(queryKeys.auth.identities())).toBeUndefined();
        expect(qc.getQueryData(queryKeys.admin.settings())).toBeUndefined();
    });

    it("keeps the session when the server did not sign out", async () => {
        // given
        const { qc } = client();
        seedAccountData(qc);
        endpoints.logout.mockRejectedValue(new Error("failed to sign out"));
        const { result } = renderHook(useLogout, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync()).rejects.toThrow("failed to sign out");
        });

        // then
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual(makeSession());
        expect(qc.getQueryData(queryKeys.auth.identities())).toEqual({ identities: [] });
    });
});

describe("useDeleteAccount", () => {
    it("sends the password and signs the browser out once the account is gone", async () => {
        // given
        const { qc } = client();
        seedAccountData(qc);

        // when
        await runMutation(useDeleteAccount, { password: "harbour-lights" }, qc);

        // then
        expect(endpoints.deleteAccount).toHaveBeenCalledWith({ password: "harbour-lights" }, expect.anything());
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual({ authenticated: false });
        expect(qc.getQueryData(queryKeys.auth.identities())).toBeUndefined();
    });
});

describe("useUpdateProfile", () => {
    it("writes the returned user into the cached session", async () => {
        // given
        const { qc, invalidate } = client();
        qc.setQueryData(queryKeys.auth.session(), makeSession({ permissions: ["view_admin_panel"] }));
        const renamed = makeSessionUser({ display_name: "Robin" });
        endpoints.updateProfile.mockResolvedValue(renamed);

        // when
        await runMutation(useUpdateProfile, { display_name: "Robin" }, qc);

        // then
        expect(endpoints.updateProfile).toHaveBeenCalledWith({ display_name: "Robin" }, expect.anything());
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual(
            makeSession({ user: renamed, permissions: ["view_admin_panel"] }),
        );
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("leaves a signed out session signed out", async () => {
        // given
        const { qc } = client();
        qc.setQueryData(queryKeys.auth.session(), { authenticated: false });
        endpoints.updateProfile.mockResolvedValue(makeSessionUser());

        // when
        await runMutation(useUpdateProfile, { display_name: "Robin" }, qc);

        // then
        expect(qc.getQueryData(queryKeys.auth.session())).toEqual({ authenticated: false });
    });
});

describe("email and password mutations", () => {
    it("verifies the address with the token and reloads the session", async () => {
        // given
        const { qc } = client();

        // when
        await runMutation(useVerifyEmail, { token: "verify-1" }, qc);

        // then
        expect(endpoints.verifyEmail).toHaveBeenCalledWith({ token: "verify-1" }, expect.anything());
        expectInvalidated(qc, queryKeys.auth.session());
    });

    it("finishes a reset with the token and reloads the session the reset ended", async () => {
        // given
        const { qc } = client();

        // when
        await runMutation(useResetPassword, { token: "reset-1", new_password: "harbour-lights" }, qc);

        // then
        expect(endpoints.resetPassword).toHaveBeenCalledWith(
            { token: "reset-1", new_password: "harbour-lights" },
            expect.anything(),
        );
        expectInvalidated(qc, queryKeys.auth.session());
    });

    it("asks for another verification link for the address, with the turnstile token, and touches no cache", async () => {
        // given
        const { qc, invalidate } = client();

        // when
        await runMutation(useResendVerification, { email: "sam@example.test", turnstile_token: "t-1" }, qc);

        // then
        expect(endpoints.resendVerification).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: "t-1" },
            expect.anything(),
        );
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("asks for a reset link with the turnstile token and touches no cache", async () => {
        // given
        const { qc, invalidate } = client();

        // when
        await runMutation(useForgotPassword, { email: "sam@example.test", turnstile_token: "t-1" }, qc);

        // then
        expect(endpoints.forgotPassword).toHaveBeenCalledWith(
            { email: "sam@example.test", turnstile_token: "t-1" },
            expect.anything(),
        );
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("asks for a change of address and leaves the session as it is, because the old address stays until the link is used", async () => {
        // given
        const { qc, invalidate } = client();

        // when
        await runMutation(useSetEmail, { email: "robin@example.test", password: "harbour-lights" }, qc);

        // then
        expect(endpoints.setEmail).toHaveBeenCalledWith(
            { email: "robin@example.test", password: "harbour-lights" },
            expect.anything(),
        );
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("changes the password and touches no cache", async () => {
        // given
        const { qc, invalidate } = client();

        // when
        await runMutation(useChangePassword, { current_password: "old", new_password: "new" }, qc);

        // then
        expect(endpoints.changePassword).toHaveBeenCalledWith(
            { current_password: "old", new_password: "new" },
            expect.anything(),
        );
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("signs out the other devices and touches no cache", async () => {
        // given
        const { qc, invalidate } = client();

        // when
        await runMutation(useSignOutOtherDevices, undefined, qc);

        // then
        expect(endpoints.signOutOtherDevices).toHaveBeenCalledOnce();
        expect(invalidate).not.toHaveBeenCalled();
    });
});
