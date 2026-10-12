import { beforeEach, describe, vi } from "vitest";
import * as api from "./twoFactor";
import {
    deleteWithBodyMock,
    fetchMock,
    postMock,
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

describe("two-factor settings", () => {
    const cases: RequestCase[] = [
        {
            name: "getTwoFactorStatus reads whether two-factor is on",
            call: () => api.getTwoFactorStatus(),
            transport: fetchMock,
            request: ["/auth/two-factor"],
            strict: true,
        },
        {
            name: "beginTwoFactorEnrolment posts the current password",
            call: () => api.beginTwoFactorEnrolment({ current_password: "harbour-lights" }),
            transport: postMock,
            request: ["/auth/two-factor/enrolment", { current_password: "harbour-lights" }],
            strict: true,
        },
        {
            name: "confirmTwoFactorEnrolment posts the first code",
            call: () => api.confirmTwoFactorEnrolment({ code: "081804" }),
            transport: postMock,
            request: ["/auth/two-factor/confirmation", { code: "081804" }],
            strict: true,
        },
        {
            name: "disableTwoFactor deletes with a current code in the body",
            call: () => api.disableTwoFactor({ code: "081804" }),
            transport: deleteWithBodyMock,
            request: ["/auth/two-factor", { code: "081804" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("two-factor sign-in", () => {
    const cases: RequestCase[] = [
        {
            name: "verifyTwoFactor posts the code for the pending sign-in and whether to trust the device",
            call: () => api.verifyTwoFactor({ code: "081804", trust_device: true }),
            transport: postMock,
            request: ["/auth/two-factor/verify", { code: "081804", trust_device: true }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
