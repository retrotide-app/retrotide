import { beforeEach, describe, vi } from "vitest";
import * as api from "./identity";
import { deleteMock, fetchMock, postMock, resetTransports, runRequestCases, type RequestCase } from "./testHarness";

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

describe("provider sign-in", () => {
    const cases: RequestCase[] = [
        {
            name: "startProviderSignIn posts a sign in with the page to come back to",
            call: () => api.startProviderSignIn("google", { purpose: "sign_in", return_to: "/account" }),
            transport: postMock,
            request: ["/auth/google/start", { purpose: "sign_in", return_to: "/account" }],
            strict: true,
        },
        {
            name: "startProviderSignIn posts a link with the current password",
            call: () => api.startProviderSignIn("google", { purpose: "link", current_password: "harbour-lights" }),
            transport: postMock,
            request: ["/auth/google/start", { purpose: "link", current_password: "harbour-lights" }],
            strict: true,
        },
        {
            name: "startProviderSignIn starts at the route of the provider it is given",
            call: () => api.startProviderSignIn("example", { purpose: "sign_in" }),
            transport: postMock,
            request: ["/auth/example/start", { purpose: "sign_in" }],
            strict: true,
        },
        {
            name: "startProviderSignIn keeps an odd provider id inside its own path segment",
            call: () => api.startProviderSignIn("a/b?c", { purpose: "sign_in" }),
            transport: postMock,
            request: ["/auth/a%2Fb%3Fc/start", { purpose: "sign_in" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("linked identities", () => {
    const cases: RequestCase[] = [
        {
            name: "listIdentities reads the account's sign-in methods",
            call: () => api.listIdentities(),
            transport: fetchMock,
            request: ["/auth/identities"],
            strict: true,
        },
        {
            name: "unlinkIdentity deletes the provider's identity",
            call: () => api.unlinkIdentity("google"),
            transport: deleteMock,
            request: ["/auth/identities/google"],
            strict: true,
        },
        {
            name: "unlinkIdentity deletes the identity of whichever provider it is given",
            call: () => api.unlinkIdentity("example"),
            transport: deleteMock,
            request: ["/auth/identities/example"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
