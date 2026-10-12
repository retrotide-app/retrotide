import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "./auth";
import {
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

describe("signing in and out", () => {
    const cases: RequestCase[] = [
        {
            name: "register posts the new account with its turnstile token",
            call: () =>
                api.register({
                    email: "sam@example.test",
                    password: "harbour-lights",
                    display_name: "Sam",
                    turnstile_token: "turnstile-1",
                }),
            transport: postMock,
            request: [
                "/auth/register",
                {
                    email: "sam@example.test",
                    password: "harbour-lights",
                    display_name: "Sam",
                    turnstile_token: "turnstile-1",
                },
            ],
            strict: true,
        },
        {
            name: "login posts the email, the password, whether to remember the sign-in and the turnstile token",
            call: () =>
                api.login({
                    email: "sam@example.test",
                    password: "harbour-lights",
                    remember: true,
                    turnstile_token: "t-1",
                }),
            transport: postMock,
            request: [
                "/auth/login",
                { email: "sam@example.test", password: "harbour-lights", remember: true, turnstile_token: "t-1" },
            ],
            strict: true,
        },
        {
            name: "logout posts with no body",
            call: () => api.logout(),
            transport: postMock,
            request: ["/auth/logout"],
            strict: true,
        },
        {
            name: "getSession reads the current session",
            call: () => api.getSession(),
            transport: fetchMock,
            request: ["/auth/session"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("email verification and password recovery", () => {
    const cases: RequestCase[] = [
        {
            name: "verifyEmail posts the token from the link",
            call: () => api.verifyEmail({ token: "verify-1" }),
            transport: postMock,
            request: ["/auth/verify-email", { token: "verify-1" }],
            strict: true,
        },
        {
            name: "resendVerification posts the address and the turnstile token, with no session needed",
            call: () => api.resendVerification({ email: "sam@example.test", turnstile_token: "t-1" }),
            transport: postMock,
            request: ["/auth/resend-verification", { email: "sam@example.test", turnstile_token: "t-1" }],
            strict: true,
        },
        {
            name: "forgotPassword posts the address and the turnstile token",
            call: () => api.forgotPassword({ email: "sam@example.test", turnstile_token: "t-1" }),
            transport: postMock,
            request: ["/auth/forgot-password", { email: "sam@example.test", turnstile_token: "t-1" }],
            strict: true,
        },
        {
            name: "resetPassword posts the token and the new password",
            call: () => api.resetPassword({ token: "reset-1", new_password: "harbour-lights" }),
            transport: postMock,
            request: ["/auth/reset-password", { token: "reset-1", new_password: "harbour-lights" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("invitations", () => {
    const cases: RequestCase[] = [
        {
            name: "getInvitation posts the token from the link, so it never lands in a server log",
            call: () => api.getInvitation({ token: "invite-1" }),
            transport: postMock,
            request: ["/auth/invitation", { token: "invite-1" }],
            strict: true,
        },
        {
            name: "acceptInvitation posts the token, the display name and the password",
            call: () => api.acceptInvitation({ token: "invite-1", display_name: "Robin", password: "harbour-lights" }),
            transport: postMock,
            request: [
                "/auth/invitation/accept",
                { token: "invite-1", display_name: "Robin", password: "harbour-lights" },
            ],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("account settings", () => {
    const cases: RequestCase[] = [
        {
            name: "setEmail posts the new address with the current password",
            call: () => api.setEmail({ email: "robin@example.test", password: "harbour-lights" }),
            transport: postMock,
            request: ["/auth/set-email", { email: "robin@example.test", password: "harbour-lights" }],
            strict: true,
        },
        {
            name: "changePassword puts the current and the new password",
            call: () => api.changePassword({ current_password: "old-pw", new_password: "new-pw" }),
            transport: putMock,
            request: ["/auth/password", { current_password: "old-pw", new_password: "new-pw" }],
            strict: true,
        },
        {
            name: "updateProfile puts the display name",
            call: () => api.updateProfile({ display_name: "Robin" }),
            transport: putMock,
            request: ["/auth/profile", { display_name: "Robin" }],
            strict: true,
        },
        {
            name: "signOutOtherDevices posts with no body",
            call: () => api.signOutOtherDevices(),
            transport: postMock,
            request: ["/auth/sign-out-other-devices"],
            strict: true,
        },
        {
            name: "deleteAccount deletes with the password in the body",
            call: () => api.deleteAccount({ password: "harbour-lights" }),
            transport: deleteWithBodyMock,
            request: ["/auth/account", { password: "harbour-lights" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("login", () => {
    it("hands back a two-factor challenge as the server sent it", async () => {
        // given
        postMock.mockResolvedValue({ two_factor_required: true });

        // when
        const response = await api.login({ email: "sam@example.test", password: "harbour-lights", remember: false });

        // then
        expect(response).toEqual({ two_factor_required: true });
    });
});
