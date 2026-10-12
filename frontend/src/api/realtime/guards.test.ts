import { describe, expect, it } from "vitest";
import {
    REALTIME_EVENT_GUARDS,
    isBoardEvent,
    isBoardEventsFrame,
    isBoardJoinedPayload,
    isBoardMePayload,
    isBoardPresencePayload,
    isBoardRefusalPayload,
    isErrorPayload,
    isPongPayload,
    isTeamActionItemsChangedPayload,
    passesRealtimeGuard,
} from "./guards";
import type { RealtimeEvent } from "./events";

const SERVER_NOW = "2026-09-22T10:06:02.512Z";

const BOARD = "8f0c";

const CARD = {
    id: "c1e2",
    column_id: "a7b1",
    parent_card_id: null,
    position: "a0V",
    body: "Deploys were smooth",
    colour: null,
    author: { participant_id: "p9d3", display_name: "Sam" },
    is_mine: true,
    hidden: true,
    votes: { total: 0 },
    created_at: "2026-09-22T10:01:02.512Z",
};

const COLUMN = {
    id: "a7b1",
    title: "Went well",
    description: "",
    colour: "green",
    position: "a0",
    max_votes: null,
    cards_revealed: false,
    hidden: false,
};

const SPOTLIGHT = { presenting_card_id: "c1e2", highlighted_card_id: null, focused_column_id: "a7b1" };

const COMMENT = {
    id: "m1f0",
    card_id: "c1e2",
    body: "Agreed",
    author: { participant_id: "p9d3", display_name: "Sam" },
    is_mine: true,
    created_at: "2026-09-22T10:03:02.512Z",
};

const SETTINGS = {
    voting_open: true,
    vote_counts_hidden: false,
    max_votes: 6,
    max_votes_per_card: 0,
    comments_enabled: true,
    merging_enabled: true,
    sort_mode: "manual",
    cards_locked: false,
    moves_locked: true,
    board_locked: false,
    timer_play_sound: true,
    timer_auto_lock: false,
    sort_enforced: true,
    anyone_can_edit: false,
    show_card_dates: false,
};

const SURVEY = {
    id: "s3e4",
    title: "Sprint health",
    template_key: "health_check",
    status: "closed",
    questions: [
        {
            id: "q4f5",
            kind: "scale",
            prompt: "How was the pace?",
            options: [],
            scale_min: 1,
            scale_max: 5,
            results: { counts: [0, 0, 0, 1, 2], responses: 3, average: 4.67 },
        },
        {
            id: "q5a6",
            kind: "single_choice",
            prompt: "Did we hit the goal?",
            options: ["Yes", "No"],
            scale_min: null,
            scale_max: null,
            results: null,
        },
    ],
    respondents: 3,
    my_answers: { q4f5: [4] },
};

const POLISHED_CARD = {
    ...CARD,
    hidden: false,
    pinned: true,
    image_path: "/uploads/8f0c/0a1b.webp",
    reactions: [{ reaction: "heart", count: 2 }],
    my_reactions: ["heart"],
};

const TIMER = { duration_ms: 300000, ends_at: "2026-09-22T10:11:02.512Z", paused_remaining_ms: null };

const ACTION_ITEM = {
    id: "i1a2",
    title: "Book the room @Ali",
    assignee: { user_id: "u1", display_name: "Sam" },
    mentions: [{ user_id: "u2", display_name: "Ali" }],
    due_on: "2026-10-20",
    status: "open",
    source_card_id: "c1e2",
};

const PREVIOUS_ACTION = { ...ACTION_ITEM, source_card_id: null, review: "carried" };

const EVERY_EVENT = [
    { type: "resync", version: 1 },
    { type: "board_updated", version: 2, title: "Sprint 12", context: "Release week" },
    {
        type: "participant_upserted",
        version: 3,
        participant: { participant_id: "p9d3", display_name: "Sam", role: "participant" },
    },
    { type: "column_upserted", version: 4, column: COLUMN },
    { type: "column_moved", version: 5, column_id: "a7b1", position: "a1", rewritten: [] },
    { type: "column_deleted", version: 6, column_id: "a7b1" },
    { type: "column_cleared", version: 7, column_id: "a7b1" },
    { type: "card_upserted", version: 8, card: CARD },
    { type: "card_moved", version: 9, card_id: "c1e2", column_id: "a7b1", position: "a2", rewritten: [] },
    { type: "card_deleted", version: 10, card_id: "c1e2" },
    { type: "hidden_counts", version: 11, hidden_counts: { a7b1: 2 } },
    { type: "settings_changed", version: 12, settings: SETTINGS },
    { type: "votes_changed", version: 13, card_id: "c1e2", total: 2 },
    { type: "votes_changed", version: 14, card_id: "c1e2", total: null },
    { type: "votes_reset", version: 15 },
    { type: "comment_upserted", version: 16, comment: COMMENT },
    { type: "comment_deleted", version: 17, comment_id: "m1f0", card_id: "c1e2" },
    {
        type: "cards_merged",
        version: 18,
        card_id: "c1e2",
        parent_card_id: "d4f7",
        column_id: "a7b1",
        position: "a3",
        restacked: ["e5a1"],
    },
    { type: "card_unmerged", version: 19, card_id: "c1e2", column_id: "a7b1", position: "a1", rewritten: [] },
    { type: "participant_removed", version: 20, participant_id: "p9d3" },
    {
        type: "cards_revealed",
        version: 21,
        stage: "vote",
        cards: [{ ...CARD, is_mine: undefined, hidden: false }],
        comments: [{ ...COMMENT, is_mine: undefined }],
    },
    { type: "cards_hidden", version: 22, hidden_counts: {} },
    { type: "timer_changed", version: 23, timer: TIMER, server_now: SERVER_NOW },
    { type: "stage_changed", version: 24, stage: "discuss" },
    { type: "action_item_upserted", version: 25, action_item: ACTION_ITEM },
    {
        type: "action_item_upserted",
        version: 26,
        action_item: { ...ACTION_ITEM, assignee: null, mentions: [], due_on: null, source_card_id: null },
    },
    { type: "action_item_removed", version: 27, action_item_id: "i1a2" },
    { type: "previous_action_upserted", version: 28, previous_action: PREVIOUS_ACTION },
    { type: "previous_action_upserted", version: 29, previous_action: { ...PREVIOUS_ACTION, review: null } },
    {
        type: "column_revealed",
        version: 30,
        column_id: "a7b1",
        cards: [{ ...CARD, hidden: false }],
        comments: [COMMENT],
    },
    { type: "column_hidden", version: 31, column_id: "a7b1" },
    { type: "column_votes_reset", version: 32, column_id: "a7b1" },
    { type: "spotlight_changed", version: 33, spotlight: SPOTLIGHT },
    {
        type: "spotlight_changed",
        version: 34,
        spotlight: { presenting_card_id: null, highlighted_card_id: null, focused_column_id: null },
    },
    { type: "column_upserted", version: 35, column: { ...COLUMN, max_votes: 3, cards_revealed: true, hidden: true } },
    {
        type: "card_reactions_changed",
        version: 36,
        card_id: "c1e2",
        reactions: [{ reaction: "thumbs_up", count: 1 }],
        my_reactions: [],
    },
    { type: "card_pinned", version: 37, card_id: "c1e2", pinned: false },
    { type: "card_image_changed", version: 38, card_id: "c1e2", image_path: "/uploads/8f0c/0a1b.webp" },
    { type: "card_image_changed", version: 39, card_id: "c1e2", image_path: null },
    { type: "board_background_changed", version: 40, background_path: "/uploads/8f0c/1c2d.webp" },
    { type: "board_background_changed", version: 41, background_path: null },
    { type: "survey_upserted", version: 42, survey: SURVEY },
    { type: "survey_upserted", version: 43, survey: { ...SURVEY, status: "open", my_answers: {} } },
    { type: "survey_removed", version: 44, survey_id: "s3e4" },
    { type: "card_upserted", version: 45, card: POLISHED_CARD },
    { type: "settings_changed", version: 46, settings: { ...SETTINGS, show_card_dates: true } },
];

function frame(overrides: Record<string, unknown> = {}) {
    return { board_id: BOARD, from: 1042, to: 1043, events: [], ...overrides };
}

describe("isPongPayload", () => {
    it("accepts what the server sends, a nonce and the server's clock", () => {
        // then
        expect(isPongPayload({ nonce: 4, server_now: SERVER_NOW })).toBe(true);
        expect(isPongPayload({ nonce: 0, server_now: SERVER_NOW })).toBe(true);
    });

    it("accepts a pong without a nonce, which answers a keepalive ping that carried none", () => {
        // then
        expect(isPongPayload({ server_now: SERVER_NOW })).toBe(true);
        expect(isPongPayload({})).toBe(true);
    });

    it("accepts a pong without the server's clock", () => {
        // then
        expect(isPongPayload({ nonce: 4 })).toBe(true);
    });

    it("rejects a nonce that is not a finite number, because it is matched against the ping that sent it", () => {
        // then
        expect(isPongPayload({ nonce: "4", server_now: SERVER_NOW })).toBe(false);
        expect(isPongPayload({ nonce: null, server_now: SERVER_NOW })).toBe(false);
        expect(isPongPayload({ nonce: Number.NaN, server_now: SERVER_NOW })).toBe(false);
        expect(isPongPayload({ nonce: Number.POSITIVE_INFINITY, server_now: SERVER_NOW })).toBe(false);
    });

    it("rejects a server clock that is not a string, because the timer's clock offset is derived from it", () => {
        // then
        expect(isPongPayload({ nonce: 4, server_now: 1727000000000 })).toBe(false);
        expect(isPongPayload({ nonce: 4, server_now: null })).toBe(false);
        expect(isPongPayload({ nonce: 4, server_now: { iso: SERVER_NOW } })).toBe(false);
    });

    it("rejects a payload that is not an object", () => {
        // then
        expect(isPongPayload(null)).toBe(false);
        expect(isPongPayload(undefined)).toBe(false);
        expect(isPongPayload("pong")).toBe(false);
        expect(isPongPayload(7)).toBe(false);
        expect(isPongPayload([{ nonce: 4 }])).toBe(false);
    });
});

describe("isErrorPayload", () => {
    it("accepts the server's error frame and rejects anything without its message", () => {
        // then
        expect(isErrorPayload({ error: "internal error" })).toBe(true);
        expect(isErrorPayload({})).toBe(false);
        expect(isErrorPayload({ error: 500 })).toBe(false);
        expect(isErrorPayload(null)).toBe(false);
    });
});

describe("isBoardJoinedPayload", () => {
    it("accepts a join answer for a seated participant and for a member not yet seated", () => {
        // then
        expect(
            isBoardJoinedPayload({
                board_id: BOARD,
                version: 1041,
                participant_id: "p9d3",
                role: "participant",
            }),
        ).toBe(true);
        expect(isBoardJoinedPayload({ board_id: BOARD, version: 0, participant_id: null, role: "facilitator" })).toBe(
            true,
        );
    });

    it("rejects a missing board, a version that is not a whole number and an unknown role", () => {
        // then
        expect(isBoardJoinedPayload({ id: BOARD, version: 1, participant_id: null, role: "participant" })).toBe(false);
        expect(isBoardJoinedPayload({ board_id: BOARD, version: 1.5, participant_id: null, role: "participant" })).toBe(
            false,
        );
        expect(isBoardJoinedPayload({ board_id: BOARD, version: -1, participant_id: null, role: "participant" })).toBe(
            false,
        );
        expect(isBoardJoinedPayload({ board_id: BOARD, version: 1, participant_id: null, role: "owner" })).toBe(false);
        expect(isBoardJoinedPayload({ board_id: "", version: 1, participant_id: null, role: "participant" })).toBe(
            false,
        );
    });
});

describe("isBoardRefusalPayload", () => {
    it.each(["not_participant", "password_required", "account_required", "link_disabled", "removed", "archived"])(
        "accepts the %s reason",
        reason => {
            // then
            expect(isBoardRefusalPayload({ board_id: BOARD, reason })).toBe(true);
        },
    );

    it("rejects a reason the client cannot explain", () => {
        // then
        expect(isBoardRefusalPayload({ board_id: BOARD, reason: "board_busy" })).toBe(false);
        expect(isBoardRefusalPayload({ board_id: BOARD })).toBe(false);
    });
});

describe("isBoardPresencePayload", () => {
    it("accepts viewers with and without their activity, which is left out while cards are hidden on an anonymous board", () => {
        // then
        expect(
            isBoardPresencePayload({
                board_id: BOARD,
                viewers: [
                    { participant_id: "p1", display_name: "Ali", state: "active" },
                    { participant_id: "p2", display_name: "Sam" },
                ],
            }),
        ).toBe(true);
        expect(isBoardPresencePayload({ board_id: BOARD, viewers: [] })).toBe(true);
    });

    it("rejects an unknown activity and a viewer list that is not a list", () => {
        // then
        expect(
            isBoardPresencePayload({
                board_id: BOARD,
                viewers: [{ participant_id: "p1", display_name: "Ali", state: "typing" }],
            }),
        ).toBe(false);
        expect(isBoardPresencePayload({ board_id: BOARD, viewers: null })).toBe(false);
    });
});

describe("isBoardEvent", () => {
    it.each(EVERY_EVENT)("accepts the $type event the server sends", event => {
        // then
        expect(isBoardEvent(event)).toBe(true);
    });

    it("accepts a card with no author and no dates, as an anonymous board sends it", () => {
        // given
        const anonymous = { ...CARD, author: null, is_mine: undefined, created_at: undefined };

        // then
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: anonymous })).toBe(true);
    });

    it("accepts a card and a comment carrying the people they mention, and rejects a mention it cannot draw", () => {
        // given
        const mentions = [{ user_id: "u1", display_name: "Sam" }];

        // then
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, mentions } })).toBe(true);
        expect(isBoardEvent({ type: "comment_upserted", version: 1, comment: { ...COMMENT, mentions } })).toBe(true);
        expect(
            isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, mentions: [{ user_id: "" }] } }),
        ).toBe(false);
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, mentions: null } })).toBe(false);
        expect(isBoardEvent({ type: "comment_upserted", version: 1, comment: { ...COMMENT, mentions: "Sam" } })).toBe(
            false,
        );
    });

    it("accepts an action item marked as the viewer's own, and rejects any other value for it", () => {
        // then
        expect(
            isBoardEvent({ type: "action_item_upserted", version: 1, action_item: { ...ACTION_ITEM, is_mine: true } }),
        ).toBe(true);
        expect(
            isBoardEvent({ type: "action_item_upserted", version: 1, action_item: { ...ACTION_ITEM, is_mine: false } }),
        ).toBe(false);
    });

    it("rejects an event type the client cannot apply, because skipping it would leave the board wrong", () => {
        // then
        expect(isBoardEvent({ type: "action_item_upserted", version: 1 })).toBe(false);
        expect(isBoardEvent({ type: "toString", version: 1 })).toBe(false);
    });

    it("accepts a card whose total is hidden, and rejects one with no votes or a broken total", () => {
        // then
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, votes: { total: null } } })).toBe(
            true,
        );
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, votes: undefined } })).toBe(false);
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, votes: { total: -1 } } })).toBe(
            false,
        );
    });

    it("accepts a comment with no author and no dates, as an anonymous board sends it", () => {
        // given
        const anonymous = { ...COMMENT, author: null, is_mine: undefined, created_at: undefined };

        // then
        expect(isBoardEvent({ type: "comment_upserted", version: 1, comment: anonymous })).toBe(true);
    });

    it("rejects a comment, a vote or settings whose fields have the wrong shape", () => {
        // then
        expect(isBoardEvent({ type: "comment_upserted", version: 1, comment: { ...COMMENT, is_mine: false } })).toBe(
            false,
        );
        expect(isBoardEvent({ type: "comment_upserted", version: 1, comment: { ...COMMENT, card_id: "" } })).toBe(
            false,
        );
        expect(isBoardEvent({ type: "votes_changed", version: 1, card_id: "c1", total: "2" })).toBe(false);
        expect(isBoardEvent({ type: "votes_changed", version: 1, card_id: "c1" })).toBe(false);
        expect(
            isBoardEvent({ type: "settings_changed", version: 1, settings: { ...SETTINGS, sort_mode: "random" } }),
        ).toBe(false);
        expect(isBoardEvent({ type: "settings_changed", version: 1, settings: { ...SETTINGS, max_votes: 1.5 } })).toBe(
            false,
        );
    });

    it("rejects a merge without the cards it restacked", () => {
        // then
        expect(
            isBoardEvent({
                type: "cards_merged",
                version: 1,
                card_id: "c1",
                parent_card_id: "c2",
                column_id: "a1",
                position: "a0",
            }),
        ).toBe(false);
    });

    it("rejects an event without a whole version", () => {
        // then
        expect(isBoardEvent({ type: "card_deleted", card_id: "c1" })).toBe(false);
        expect(isBoardEvent({ type: "card_deleted", version: "4", card_id: "c1" })).toBe(false);
    });

    it("rejects a card whose fields have the wrong shape", () => {
        // then
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, hidden: "yes" } })).toBe(false);
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, is_mine: false } })).toBe(false);
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, colour: "mauve" } })).toBe(false);
        expect(
            isBoardEvent({ type: "card_upserted", version: 1, card: { ...CARD, author: { display_name: "x" } } }),
        ).toBe(false);
    });

    it("rejects a move whose rewritten keys are missing or malformed", () => {
        // then
        expect(isBoardEvent({ type: "card_moved", version: 1, card_id: "c1", column_id: "a1", position: "a0" })).toBe(
            false,
        );
        expect(
            isBoardEvent({
                type: "column_moved",
                version: 1,
                column_id: "a1",
                position: "a0",
                rewritten: [{ id: "a2" }],
            }),
        ).toBe(false);
    });

    it("rejects counts that are not whole and non-negative", () => {
        // then
        expect(isBoardEvent({ type: "hidden_counts", version: 1, hidden_counts: { a1: -1 } })).toBe(false);
        expect(isBoardEvent({ type: "hidden_counts", version: 1, hidden_counts: { a1: 0.5 } })).toBe(false);
        expect(isBoardEvent({ type: "hidden_counts", version: 1, hidden_counts: [] })).toBe(false);
    });

    it("rejects settings that leave out a lock or the timer sound", () => {
        // given
        const { board_locked: _locked, ...withoutLock } = SETTINGS;

        // then
        expect(isBoardEvent({ type: "settings_changed", version: 1, settings: withoutLock })).toBe(false);
        expect(
            isBoardEvent({ type: "settings_changed", version: 1, settings: { ...SETTINGS, timer_play_sound: "on" } }),
        ).toBe(false);
    });

    it("accepts a running timer, a paused timer and a stopped one", () => {
        // then
        expect(isBoardEvent({ type: "timer_changed", version: 1, timer: TIMER, server_now: SERVER_NOW })).toBe(true);
        expect(
            isBoardEvent({
                type: "timer_changed",
                version: 1,
                timer: { duration_ms: 300000, ends_at: null, paused_remaining_ms: 120000 },
                server_now: SERVER_NOW,
            }),
        ).toBe(true);
        expect(isBoardEvent({ type: "timer_changed", version: 1, timer: null, server_now: SERVER_NOW })).toBe(true);
    });

    it("rejects a timer that is both running and paused, or neither, or comes without the server's clock", () => {
        // then
        expect(
            isBoardEvent({
                type: "timer_changed",
                version: 1,
                timer: { ...TIMER, paused_remaining_ms: 1000 },
                server_now: SERVER_NOW,
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "timer_changed",
                version: 1,
                timer: { duration_ms: 300000, ends_at: null, paused_remaining_ms: null },
                server_now: SERVER_NOW,
            }),
        ).toBe(false);
        expect(isBoardEvent({ type: "timer_changed", version: 1, timer: TIMER })).toBe(false);
        expect(isBoardEvent({ type: "timer_changed", version: 1, timer: TIMER, server_now: "soon" })).toBe(false);
        expect(
            isBoardEvent({
                type: "timer_changed",
                version: 1,
                timer: { ...TIMER, ends_at: "later" },
                server_now: SERVER_NOW,
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "timer_changed",
                version: 1,
                timer: { ...TIMER, duration_ms: -1 },
                server_now: SERVER_NOW,
            }),
        ).toBe(false);
    });

    it("rejects a reveal whose stage, cards or comments are missing or malformed", () => {
        // then
        expect(isBoardEvent({ type: "cards_revealed", version: 1, stage: "vote", cards: [CARD] })).toBe(false);
        expect(
            isBoardEvent({
                type: "cards_revealed",
                version: 1,
                stage: "vote",
                cards: [{ ...CARD, hidden: "no" }],
                comments: [],
            }),
        ).toBe(false);
        expect(
            isBoardEvent({ type: "cards_revealed", version: 1, stage: "vote", cards: [], comments: [{ id: "m1" }] }),
        ).toBe(false);
        expect(isBoardEvent({ type: "cards_revealed", version: 1, cards: [], comments: [] })).toBe(false);
        expect(isBoardEvent({ type: "cards_revealed", version: 1, stage: "review", cards: [], comments: [] })).toBe(
            false,
        );
    });

    it("rejects a stage change without a stage or with one that is not among the five", () => {
        // then
        expect(isBoardEvent({ type: "stage_changed", version: 1 })).toBe(false);
        expect(isBoardEvent({ type: "stage_changed", version: 1, stage: "review" })).toBe(false);
        expect(isBoardEvent({ type: "stage_changed", version: 1, stage: 2 })).toBe(false);
    });

    it("rejects a re-hide with broken counts and a removal without its participant", () => {
        // then
        expect(isBoardEvent({ type: "cards_hidden", version: 1, hidden_counts: { a1: -2 } })).toBe(false);
        expect(isBoardEvent({ type: "cards_hidden", version: 1 })).toBe(false);
        expect(isBoardEvent({ type: "participant_removed", version: 1, participant_id: "" })).toBe(false);
    });

    it("rejects an action item with a status, a due date or a person it cannot draw", () => {
        // then
        expect(
            isBoardEvent({ type: "action_item_upserted", version: 1, action_item: { ...ACTION_ITEM, status: "late" } }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "action_item_upserted",
                version: 1,
                action_item: { ...ACTION_ITEM, due_on: "20 October" },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "action_item_upserted",
                version: 1,
                action_item: { ...ACTION_ITEM, assignee: { user_id: "" } },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({ type: "action_item_upserted", version: 1, action_item: { ...ACTION_ITEM, mentions: null } }),
        ).toBe(false);
        expect(isBoardEvent({ type: "action_item_upserted", version: 1 })).toBe(false);
    });

    it("rejects a column without its vote limit, reveal or hidden flag", () => {
        // given
        const { hidden: _hidden, ...withoutHidden } = COLUMN;

        // then
        expect(isBoardEvent({ type: "column_upserted", version: 1, column: withoutHidden })).toBe(false);
        expect(isBoardEvent({ type: "column_upserted", version: 1, column: { ...COLUMN, max_votes: -1 } })).toBe(false);
        expect(isBoardEvent({ type: "column_upserted", version: 1, column: { ...COLUMN, max_votes: "3" } })).toBe(
            false,
        );
        expect(isBoardEvent({ type: "column_upserted", version: 1, column: { ...COLUMN, cards_revealed: 1 } })).toBe(
            false,
        );
    });

    it("rejects settings that leave out auto-lock, the enforced sort or anyone can edit", () => {
        // given
        const { anyone_can_edit: _anyone, ...withoutAnyone } = SETTINGS;

        // then
        expect(isBoardEvent({ type: "settings_changed", version: 1, settings: withoutAnyone })).toBe(false);
        expect(
            isBoardEvent({ type: "settings_changed", version: 1, settings: { ...SETTINGS, timer_auto_lock: "yes" } }),
        ).toBe(false);
        expect(
            isBoardEvent({ type: "settings_changed", version: 1, settings: { ...SETTINGS, sort_enforced: null } }),
        ).toBe(false);
    });

    it("rejects a column event without its column and a revealed column with broken cards", () => {
        // then
        expect(isBoardEvent({ type: "column_hidden", version: 1 })).toBe(false);
        expect(isBoardEvent({ type: "column_votes_reset", version: 1, column_id: "" })).toBe(false);
        expect(isBoardEvent({ type: "column_revealed", version: 1, column_id: "a7b1", cards: [] })).toBe(false);
        expect(
            isBoardEvent({
                type: "column_revealed",
                version: 1,
                column_id: "a7b1",
                cards: [{ ...CARD, hidden: "no" }],
                comments: [],
            }),
        ).toBe(false);
    });

    it("rejects a spotlight that is missing or names something it cannot read", () => {
        // given
        const { focused_column_id: _focused, ...withoutFocus } = SPOTLIGHT;

        // then
        expect(isBoardEvent({ type: "spotlight_changed", version: 1 })).toBe(false);
        expect(isBoardEvent({ type: "spotlight_changed", version: 1, spotlight: withoutFocus })).toBe(false);
        expect(
            isBoardEvent({
                type: "spotlight_changed",
                version: 1,
                spotlight: { ...SPOTLIGHT, presenting_card_id: "" },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "spotlight_changed",
                version: 1,
                spotlight: { ...SPOTLIGHT, highlighted_card_id: 4 },
            }),
        ).toBe(false);
    });

    it("rejects a removal without its item and a previous action with an unknown review", () => {
        // then
        expect(isBoardEvent({ type: "action_item_removed", version: 1, action_item_id: "" })).toBe(false);
        expect(
            isBoardEvent({
                type: "previous_action_upserted",
                version: 1,
                previous_action: { ...PREVIOUS_ACTION, review: "later" },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({ type: "previous_action_upserted", version: 1, previous_action: { ...ACTION_ITEM } }),
        ).toBe(false);
    });

    it("rejects a card whose image path is not a server upload, or whose pin or reactions it cannot read", () => {
        // then
        expect(
            isBoardEvent({
                type: "card_upserted",
                version: 1,
                card: { ...POLISHED_CARD, image_path: "https://evil.example/x.png" },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "card_upserted",
                version: 1,
                card: { ...POLISHED_CARD, image_path: "javascript:alert(1)" },
            }),
        ).toBe(false);
        expect(isBoardEvent({ type: "card_upserted", version: 1, card: { ...POLISHED_CARD, pinned: false } })).toBe(
            false,
        );
        expect(
            isBoardEvent({
                type: "card_upserted",
                version: 1,
                card: { ...POLISHED_CARD, reactions: [{ reaction: "skull", count: 1 }] },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "card_upserted",
                version: 1,
                card: { ...POLISHED_CARD, reactions: [{ reaction: "heart", count: -1 }] },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({ type: "card_upserted", version: 1, card: { ...POLISHED_CARD, my_reactions: ["skull"] } }),
        ).toBe(false);
    });

    it("rejects settings without the card date switch", () => {
        // given
        const { show_card_dates: _left, ...withoutDates } = SETTINGS;

        // then
        expect(isBoardEvent({ type: "settings_changed", version: 1, settings: withoutDates })).toBe(false);
    });

    it("rejects reaction, pin, image and background events with broken fields", () => {
        // then
        expect(isBoardEvent({ type: "card_reactions_changed", version: 1, card_id: "c1e2", reactions: [] })).toBe(
            false,
        );
        expect(
            isBoardEvent({ type: "card_reactions_changed", version: 1, card_id: "", reactions: [], my_reactions: [] }),
        ).toBe(false);
        expect(isBoardEvent({ type: "card_pinned", version: 1, card_id: "c1e2" })).toBe(false);
        expect(
            isBoardEvent({ type: "card_image_changed", version: 1, card_id: "c1e2", image_path: "/static/x.webp" }),
        ).toBe(false);
        expect(isBoardEvent({ type: "card_image_changed", version: 1, card_id: "c1e2" })).toBe(false);
        expect(
            isBoardEvent({
                type: "board_background_changed",
                version: 1,
                background_path: "data:image/png;base64,AAAA",
            }),
        ).toBe(false);
        expect(isBoardEvent({ type: "board_background_changed", version: 1 })).toBe(false);
    });

    it("rejects a survey it cannot draw", () => {
        // then
        expect(isBoardEvent({ type: "survey_upserted", version: 1, survey: { ...SURVEY, status: "draft" } })).toBe(
            false,
        );
        expect(isBoardEvent({ type: "survey_upserted", version: 1, survey: { ...SURVEY, my_answers: null } })).toBe(
            false,
        );
        expect(
            isBoardEvent({ type: "survey_upserted", version: 1, survey: { ...SURVEY, my_answers: { q4f5: ["4"] } } }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "survey_upserted",
                version: 1,
                survey: { ...SURVEY, questions: [{ ...SURVEY.questions[0], kind: "text" }] },
            }),
        ).toBe(false);
        expect(
            isBoardEvent({
                type: "survey_upserted",
                version: 1,
                survey: { ...SURVEY, questions: [{ ...SURVEY.questions[0], results: { counts: [1], responses: 1 } }] },
            }),
        ).toBe(false);
        expect(isBoardEvent({ type: "survey_removed", version: 1, survey_id: "" })).toBe(false);
    });
});

describe("isTeamActionItemsChangedPayload", () => {
    it("accepts the team whose action items changed and rejects a frame without one", () => {
        // then
        expect(isTeamActionItemsChangedPayload({ team_id: "t1" })).toBe(true);
        expect(isTeamActionItemsChangedPayload({ team_id: "" })).toBe(false);
        expect(isTeamActionItemsChangedPayload({})).toBe(false);
        expect(isTeamActionItemsChangedPayload(null)).toBe(false);
    });
});

describe("isBoardEventsFrame", () => {
    it("accepts a frame whose events rise inside its range", () => {
        // given
        const events = [
            { type: "card_upserted", version: 1042, card: CARD },
            { type: "hidden_counts", version: 1043, hidden_counts: { a7b1: 1 } },
        ];

        // then
        expect(isBoardEventsFrame(frame({ events }))).toBe(true);
    });

    it("accepts a frame with no events, which covers versions the viewer may not see", () => {
        // then
        expect(isBoardEventsFrame(frame())).toBe(true);
    });

    it("rejects a frame whose range runs backwards", () => {
        // then
        expect(isBoardEventsFrame(frame({ from: 9, to: 8 }))).toBe(false);
    });

    it("rejects an event outside the frame's range or out of order", () => {
        // then
        expect(isBoardEventsFrame(frame({ events: [{ type: "card_deleted", version: 1041, card_id: "c1" }] }))).toBe(
            false,
        );
        expect(isBoardEventsFrame(frame({ events: [{ type: "card_deleted", version: 1044, card_id: "c1" }] }))).toBe(
            false,
        );
        expect(
            isBoardEventsFrame(
                frame({
                    events: [
                        { type: "card_deleted", version: 1043, card_id: "c1" },
                        { type: "card_deleted", version: 1042, card_id: "c2" },
                    ],
                }),
            ),
        ).toBe(false);
        expect(
            isBoardEventsFrame(
                frame({
                    events: [
                        { type: "card_deleted", version: 1042, card_id: "c1" },
                        { type: "card_deleted", version: 1042, card_id: "c2" },
                    ],
                }),
            ),
        ).toBe(false);
    });

    it("rejects a frame holding any event it cannot apply", () => {
        // then
        expect(isBoardEventsFrame(frame({ events: [{ type: "timer_changed", version: 1042 }] }))).toBe(false);
    });

    it("rejects a frame without its board or its range", () => {
        // then
        expect(isBoardEventsFrame(frame({ board_id: undefined }))).toBe(false);
        expect(isBoardEventsFrame(frame({ from: undefined }))).toBe(false);
        expect(isBoardEventsFrame(frame({ events: undefined }))).toBe(false);
        expect(isBoardEventsFrame(null)).toBe(false);
    });
});

describe("isBoardMePayload", () => {
    it("accepts the viewer's own votes and rejects a malformed tally", () => {
        // then
        expect(isBoardMePayload({ board_id: BOARD, version: 1046, votes_remaining: 5, my_votes: { d4f7: 1 } })).toBe(
            true,
        );
        expect(isBoardMePayload({ board_id: BOARD, version: 1046, votes_remaining: 5, my_votes: { d4f7: "1" } })).toBe(
            false,
        );
    });
});

describe("the guard table", () => {
    it("guards every frame the server may send", () => {
        // then
        expect(Object.keys(REALTIME_EVENT_GUARDS).sort()).toEqual(
            [
                "board_access_revoked",
                "board_events",
                "board_join_rejected",
                "board_joined",
                "board_me",
                "board_presence",
                "error",
                "pong",
                "team_action_items_changed",
            ].sort(),
        );
        expect(REALTIME_EVENT_GUARDS.pong).toBe(isPongPayload);
    });

    it("applies the guard when the event has one", () => {
        // given
        const good: RealtimeEvent = { type: "pong", data: { nonce: 4, server_now: SERVER_NOW } };
        const bad = { type: "pong", data: { nonce: 4, server_now: 1727000000000 } } as unknown as RealtimeEvent;

        // then
        expect(passesRealtimeGuard(good)).toBe(true);
        expect(passesRealtimeGuard(bad)).toBe(false);
    });

    it("rejects an event whose payload is missing altogether", () => {
        // given
        const event = { type: "pong" } as unknown as RealtimeEvent;

        // then
        expect(passesRealtimeGuard(event)).toBe(false);
    });

    it("drops a board frame carrying a malformed event before any handler sees it", () => {
        // given
        const event = {
            type: "board_events",
            data: frame({ events: [{ type: "card_upserted", version: 1042, card: { id: "c1" } }] }),
        } as unknown as RealtimeEvent;

        // then
        expect(passesRealtimeGuard(event)).toBe(false);
    });
});
