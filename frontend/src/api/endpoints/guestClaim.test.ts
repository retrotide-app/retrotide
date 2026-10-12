import { beforeEach, describe, vi } from "vitest";
import * as api from "./guestClaim";
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

describe("the guest claim", () => {
    const cases: RequestCase[] = [
        {
            name: "getGuestClaimOffer reads the boards this browser joined as a guest",
            call: () => api.getGuestClaimOffer(),
            transport: fetchMock,
            request: ["/auth/guest-claim"],
            strict: true,
        },
        {
            name: "claimGuestBoards posts the chosen boards",
            call: () => api.claimGuestBoards({ board_ids: ["b1", "b2"] }),
            transport: postMock,
            request: ["/auth/guest-claim", { board_ids: ["b1", "b2"] }],
            strict: true,
        },
        {
            name: "declineGuestClaim deletes the offer",
            call: () => api.declineGuestClaim(),
            transport: deleteMock,
            request: ["/auth/guest-claim"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
