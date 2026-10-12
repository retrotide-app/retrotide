import { beforeEach, describe, vi } from "vitest";
import * as api from "./boards";
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

describe("a team's boards", () => {
    const cases: RequestCase[] = [
        {
            name: "listTeamBoards reads the team's boards",
            call: () => api.listTeamBoards("t1"),
            transport: fetchMock,
            request: ["/teams/t1/boards"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("an organisation's boards", () => {
    const cases: RequestCase[] = [
        {
            name: "createBoard posts the new board's title, template, settings and team to the organisation",
            call: () =>
                api.createBoard("o1", {
                    title: "Sprint 12",
                    template_key: "blank",
                    context: "Release week",
                    anonymity: "anonymous",
                    cards_shown: true,
                    team_id: "t1",
                }),
            transport: postMock,
            request: [
                "/orgs/o1/boards",
                {
                    title: "Sprint 12",
                    template_key: "blank",
                    context: "Release week",
                    anonymity: "anonymous",
                    cards_shown: true,
                    team_id: "t1",
                },
            ],
            strict: true,
        },
        {
            name: "createBoard posts a board with no team as it is, and escapes the organisation id",
            call: () => api.createBoard("o/1", { title: "Sprint 12", template_key: "blank" }),
            transport: postMock,
            request: ["/orgs/o%2F1/boards", { title: "Sprint 12", template_key: "blank" }],
            strict: true,
        },
        {
            name: "listOrganisationBoards reads every board the viewer may see in the organisation",
            call: () => api.listOrganisationBoards("o1"),
            transport: fetchMock,
            request: ["/orgs/o1/boards"],
            strict: true,
        },
        {
            name: "listOrganisationBoards escapes the organisation id so it cannot add a segment",
            call: () => api.listOrganisationBoards("o/1"),
            transport: fetchMock,
            request: ["/orgs/o%2F1/boards"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("one board", () => {
    const cases: RequestCase[] = [
        {
            name: "getBoard reads the whole board when no version is held",
            call: () => api.getBoard("b1"),
            transport: fetchMock,
            request: ["/boards/b1"],
            strict: true,
        },
        {
            name: "getBoard sends the version it holds so an unchanged board answers with nothing",
            call: () => api.getBoard("b1", 7),
            transport: fetchMock,
            request: ["/boards/b1?known_version=7"],
            strict: true,
        },
        {
            name: "getBoard says it is catching up after a reconnect, beside the version it holds",
            call: () => api.getBoard("b1", 7, "reconnect"),
            transport: fetchMock,
            request: ["/boards/b1?known_version=7&reason=reconnect"],
            strict: true,
        },
        {
            name: "joinBoard posts to the viewer's own participant route with no body",
            call: () => api.joinBoard("b1"),
            transport: postMock,
            request: ["/boards/b1/participants/me"],
            strict: true,
        },
        {
            name: "describeBoard puts the board's description with its op id",
            call: () => api.describeBoard("b1", { op_id: "op1", context: "Release week" }),
            transport: putMock,
            request: ["/boards/b1/context", { op_id: "op1", context: "Release week" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a board's columns", () => {
    const cases: RequestCase[] = [
        {
            name: "createColumn posts the new column with its own id",
            call: () => api.createColumn("b1", { op_id: "op1", id: "c1", title: "Kudos", colour: "teal" }),
            transport: postMock,
            request: ["/boards/b1/columns", { op_id: "op1", id: "c1", title: "Kudos", colour: "teal" }],
            strict: true,
        },
        {
            name: "updateColumn puts the column's fields",
            call: () =>
                api.updateColumn("b1", "c1", { op_id: "op1", title: "Kudos", description: "Thanks", colour: "pink" }),
            transport: putMock,
            request: ["/boards/b1/columns/c1", { op_id: "op1", title: "Kudos", description: "Thanks", colour: "pink" }],
            strict: true,
        },
        {
            name: "moveColumn posts the column it should follow",
            call: () => api.moveColumn("b1", "c1", { op_id: "op1", after_column_id: null }),
            transport: postMock,
            request: ["/boards/b1/columns/c1/move", { op_id: "op1", after_column_id: null }],
            strict: true,
        },
        {
            name: "clearColumn posts to the column's clear route",
            call: () => api.clearColumn("b1", "c1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/columns/c1/clear", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "deleteColumn deletes the column with its op id in the body",
            call: () => api.deleteColumn("b1", "c1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/columns/c1", { op_id: "op1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a board's cards", () => {
    const cases: RequestCase[] = [
        {
            name: "createCard posts the new card with its own id and column",
            call: () =>
                api.createCard("b1", { op_id: "op1", id: "k1", column_id: "c1", body: "Ship it", colour: null }),
            transport: postMock,
            request: ["/boards/b1/cards", { op_id: "op1", id: "k1", column_id: "c1", body: "Ship it", colour: null }],
            strict: true,
        },
        {
            name: "createCard posts the people the new card mentions",
            call: () =>
                api.createCard("b1", { op_id: "op1", id: "k1", column_id: "c1", body: "Ask @Ali", mentions: ["u2"] }),
            transport: postMock,
            request: [
                "/boards/b1/cards",
                { op_id: "op1", id: "k1", column_id: "c1", body: "Ask @Ali", mentions: ["u2"] },
            ],
            strict: true,
        },
        {
            name: "editCard puts the card's new text with everyone it still mentions",
            call: () => api.editCard("b1", "k1", { op_id: "op1", body: "Ship it today @Ali", mentions: ["u2"] }),
            transport: putMock,
            request: ["/boards/b1/cards/k1", { op_id: "op1", body: "Ship it today @Ali", mentions: ["u2"] }],
            strict: true,
        },
        {
            name: "recolourCard puts the card's colour",
            call: () => api.recolourCard("b1", "k1", { op_id: "op1", colour: "amber" }),
            transport: putMock,
            request: ["/boards/b1/cards/k1/colour", { op_id: "op1", colour: "amber" }],
            strict: true,
        },
        {
            name: "moveCard posts the column and the card it should follow",
            call: () => api.moveCard("b1", "k1", { op_id: "op1", column_id: "c2", after_card_id: "k2" }),
            transport: postMock,
            request: ["/boards/b1/cards/k1/move", { op_id: "op1", column_id: "c2", after_card_id: "k2" }],
            strict: true,
        },
        {
            name: "deleteCard deletes the card with its op id in the body",
            call: () => api.deleteCard("b1", "k1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/cards/k1", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "mergeCard posts the card it should join",
            call: () => api.mergeCard("b1", "k1", { op_id: "op1", target_card_id: "k2" }),
            transport: postMock,
            request: ["/boards/b1/cards/k1/merge", { op_id: "op1", target_card_id: "k2" }],
            strict: true,
        },
        {
            name: "unmergeCard posts to the card's unmerge route",
            call: () => api.unmergeCard("b1", "k1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/cards/k1/unmerge", { op_id: "op1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a board's votes and settings", () => {
    const cases: RequestCase[] = [
        {
            name: "castVote posts a vote on the card",
            call: () => api.castVote("b1", "k1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/cards/k1/votes", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "removeVote deletes one vote from the card with its op id in the body",
            call: () => api.removeVote("b1", "k1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/cards/k1/votes", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "resetVotes posts to the board's reset route",
            call: () => api.resetVotes("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/votes/reset", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "updateBoardSettings puts only the settings it was given",
            call: () => api.updateBoardSettings("b1", { op_id: "op1", voting_open: false }),
            transport: putMock,
            request: ["/boards/b1/settings", { op_id: "op1", voting_open: false }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("facilitating a board", () => {
    const cases: RequestCase[] = [
        {
            name: "revealCards posts to the board's reveal route",
            call: () => api.revealCards("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/reveal", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "hideCards posts to the board's hide route",
            call: () => api.hideCards("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/hide", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "changeStage posts the stage to move to on the board's stage route",
            call: () => api.changeStage("b1", { op_id: "op1", stage: "vote" }),
            transport: postMock,
            request: ["/boards/b1/stage", { op_id: "op1", stage: "vote" }],
            strict: true,
        },
        {
            name: "runTimer posts a start with its length",
            call: () => api.runTimer("b1", { op_id: "op1", action: "start", duration_ms: 300000 }),
            transport: postMock,
            request: ["/boards/b1/timer", { op_id: "op1", action: "start", duration_ms: 300000 }],
            strict: true,
        },
        {
            name: "runTimer posts any other action without a length",
            call: () => api.runTimer("b1", { op_id: "op1", action: "add_minute" }),
            transport: postMock,
            request: ["/boards/b1/timer", { op_id: "op1", action: "add_minute" }],
            strict: true,
        },
        {
            name: "closeBoard posts to the board's close route",
            call: () => api.closeBoard("b1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/close", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "setParticipantRole puts the participant's new role",
            call: () => api.setParticipantRole("b1", "p1", { op_id: "op1", role: "facilitator" }),
            transport: putMock,
            request: ["/boards/b1/participants/p1/role", { op_id: "op1", role: "facilitator" }],
            strict: true,
        },
        {
            name: "removeParticipant deletes the participant with its op id in the body",
            call: () => api.removeParticipant("b1", "p1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/participants/p1", { op_id: "op1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a card's comments", () => {
    const cases: RequestCase[] = [
        {
            name: "createComment posts the new comment with its own id on the card",
            call: () => api.createComment("b1", "k1", { op_id: "op1", id: "m1", body: "Agreed" }),
            transport: postMock,
            request: ["/boards/b1/cards/k1/comments", { op_id: "op1", id: "m1", body: "Agreed" }],
            strict: true,
        },
        {
            name: "createComment posts the people the new comment mentions",
            call: () => api.createComment("b1", "k1", { op_id: "op1", id: "m1", body: "Ask @Ali", mentions: ["u2"] }),
            transport: postMock,
            request: ["/boards/b1/cards/k1/comments", { op_id: "op1", id: "m1", body: "Ask @Ali", mentions: ["u2"] }],
            strict: true,
        },
        {
            name: "editComment puts the comment's new text with everyone it still mentions",
            call: () => api.editComment("b1", "m1", { op_id: "op1", body: "Agreed, mostly", mentions: [] }),
            transport: putMock,
            request: ["/boards/b1/comments/m1", { op_id: "op1", body: "Agreed, mostly", mentions: [] }],
            strict: true,
        },
        {
            name: "deleteComment deletes the comment with its op id in the body",
            call: () => api.deleteComment("b1", "m1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/comments/m1", { op_id: "op1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("facilitating a board's columns and spotlight", () => {
    const cases: RequestCase[] = [
        {
            name: "revealColumn posts the reveal of one column's cards",
            call: () => api.revealColumn("b1", "c1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/columns/c1/reveal", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "hideColumn posts the hiding of a column from participants",
            call: () => api.hideColumn("b1", "c1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/columns/c1/hide", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "showColumn posts the showing of a hidden column to everyone, and escapes the ids",
            call: () => api.showColumn("b/1", "c/1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b%2F1/columns/c%2F1/show", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "resetColumnVotes posts the reset of one column's votes",
            call: () => api.resetColumnVotes("b1", "c1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/columns/c1/votes/reset", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "setColumnVoteLimit puts the column's vote limit",
            call: () => api.setColumnVoteLimit("b1", "c1", { op_id: "op1", max_votes: 3 }),
            transport: putMock,
            request: ["/boards/b1/columns/c1/vote-limit", { op_id: "op1", max_votes: 3 }],
            strict: true,
        },
        {
            name: "setColumnVoteLimit puts no limit as null",
            call: () => api.setColumnVoteLimit("b1", "c1", { op_id: "op1", max_votes: null }),
            transport: putMock,
            request: ["/boards/b1/columns/c1/vote-limit", { op_id: "op1", max_votes: null }],
            strict: true,
        },
        {
            name: "presentCard posts the card to present",
            call: () => api.presentCard("b1", { op_id: "op1", card_id: "k1" }),
            transport: postMock,
            request: ["/boards/b1/presentation", { op_id: "op1", card_id: "k1" }],
            strict: true,
        },
        {
            name: "presentCard posts null to end the presentation",
            call: () => api.presentCard("b1", { op_id: "op1", card_id: null }),
            transport: postMock,
            request: ["/boards/b1/presentation", { op_id: "op1", card_id: null }],
            strict: true,
        },
        {
            name: "highlightCard posts the card to highlight",
            call: () => api.highlightCard("b1", { op_id: "op1", card_id: "k1" }),
            transport: postMock,
            request: ["/boards/b1/highlight", { op_id: "op1", card_id: "k1" }],
            strict: true,
        },
        {
            name: "focusColumn posts the column to focus",
            call: () => api.focusColumn("b1", { op_id: "op1", column_id: "c1" }),
            transport: postMock,
            request: ["/boards/b1/focus", { op_id: "op1", column_id: "c1" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
