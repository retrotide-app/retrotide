import { beforeEach, describe, vi } from "vitest";
import * as api from "./actionItems";
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

describe("a board's people", () => {
    const cases: RequestCase[] = [
        {
            name: "listBoardPeople reads who can be named on the board",
            call: () => api.listBoardPeople("b/1"),
            transport: fetchMock,
            request: ["/boards/b%2F1/people"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a board's action items", () => {
    const fields = { title: "Book the room @Ali", assignee_user_id: "u1", due_on: "2026-10-20", mentions: ["u2"] };

    const cases: RequestCase[] = [
        {
            name: "addActionItem posts the new item, its id and the card it came from",
            call: () => api.addActionItem("b1", { ...fields, op_id: "op1", id: "i1", source_card_id: "k1" }),
            transport: postMock,
            request: ["/boards/b1/action-items", { ...fields, op_id: "op1", id: "i1", source_card_id: "k1" }],
            strict: true,
        },
        {
            name: "editActionItem puts every field of the item",
            call: () => api.editActionItem("b1", "i/1", { ...fields, op_id: "op1" }),
            transport: putMock,
            request: ["/boards/b1/action-items/i%2F1", { ...fields, op_id: "op1" }],
            strict: true,
        },
        {
            name: "markActionItem posts the item's new status",
            call: () => api.markActionItem("b1", "i1", { op_id: "op1", status: "done" }),
            transport: postMock,
            request: ["/boards/b1/action-items/i1/status", { op_id: "op1", status: "done" }],
            strict: true,
        },
        {
            name: "deleteActionItem deletes the item with its op id",
            call: () => api.deleteActionItem("b1", "i1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/action-items/i1", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "reviewPreviousAction posts the outcome for an earlier board's item",
            call: () => api.reviewPreviousAction("b1", "i/9", { op_id: "op1", outcome: "carried" }),
            transport: postMock,
            request: ["/boards/b1/previous-actions/i%2F9/review", { op_id: "op1", outcome: "carried" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("the organisation's action items", () => {
    const cases: RequestCase[] = [
        {
            name: "listOrganisationActionItems reads the first page with no filter",
            call: () => api.listOrganisationActionItems("o1", {}),
            transport: fetchMock,
            request: ["/orgs/o1/action-items"],
            strict: true,
        },
        {
            name: "listOrganisationActionItems sends every filter and the page",
            call: () =>
                api.listOrganisationActionItems("o/1", {
                    status: "open",
                    assignee: "me",
                    team: "organisation",
                    limit: 50,
                    offset: 100,
                }),
            transport: fetchMock,
            request: ["/orgs/o%2F1/action-items?status=open&assignee=me&team=organisation&limit=50&offset=100"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("the signed in user's mentions", () => {
    const cases: RequestCase[] = [
        {
            name: "listMentions reads the account's mentions",
            call: () => api.listMentions(),
            transport: fetchMock,
            request: ["/auth/mentions"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
