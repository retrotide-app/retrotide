import { describe, expect, it } from "vitest";
import {
    makeApiError,
    makeBoardParticipant,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeParticipantViewer,
    VIEWER_PARTICIPANT_ID,
} from "../../test-utils/fixtures";
import { activeLocks, facilitationRefusal, participantRows } from "./facilitation";

const people = [
    makeBoardParticipant({ participant_id: "p3", display_name: "Zoe", role: "participant" }),
    makeBoardParticipant({ participant_id: "p2", display_name: "Ali", role: "participant" }),
    makeBoardParticipant({ participant_id: VIEWER_PARTICIPANT_ID, display_name: "Sam", role: "facilitator" }),
    makeBoardParticipant({ participant_id: "p4", display_name: "Bea", role: "facilitator" }),
];

describe("activeLocks", () => {
    it("lists nothing on an unlocked board", () => {
        // then
        expect(activeLocks(makeBoardSettings())).toEqual([]);
    });

    it("lists every lock that is on, the whole board first", () => {
        // when
        const locks = activeLocks(makeBoardSettings({ board_locked: true, cards_locked: true, moves_locked: true }));

        // then
        expect(locks.map(lock => lock.label)).toEqual(["Board locked", "Cards locked", "Moving locked"]);
    });
});

describe("facilitationRefusal", () => {
    it.each([
        ["cannot_promote", "Only someone signed in through the team or organisation can be a co-facilitator."],
        ["lead_facilitates", "This person facilitates through their team or organisation role"],
        ["remove_self", "You cannot remove yourself from the board."],
        ["timer_too_long", "A timer can run for an hour at most."],
        ["timer_not_running", "The timer is not running any more."],
        ["timer_not_paused", "The timer is not paused any more."],
        ["column_hidden", "That column is hidden from participants"],
        ["column_revealed", "A card cannot move between a revealed column and one whose cards are still hidden."],
        ["column_vote_limit_reached", "You have used every vote this column allows."],
        ["card_not_shown", "Only a card everyone can see can be presented or highlighted."],
        ["invalid_column_vote_limit", "A column vote limit must be between 1 and 99, or none."],
    ])("explains %s in plain words", (code, words) => {
        // when
        const message = facilitationRefusal(makeApiError({ status: 409, code, message: "refused" }));

        // then
        expect(message).toContain(words);
    });

    it("hands back the server's own message for anything else", () => {
        // when
        const message = facilitationRefusal(makeApiError({ status: 503, code: "board_busy", message: "Try again" }));

        // then
        expect(message).toBe("Try again");
    });

    it.each([
        ["wrong_stage", "votes can be cast or taken back only in the Vote stage"],
        [
            "stage_not_next",
            "a board moves one stage at a time, so it can only go to the stage straight after its current one or back to the one before it",
        ],
    ])("shows the server's own message for %s", (code, serverMessage) => {
        // when
        const message = facilitationRefusal(makeApiError({ status: 409, code, message: serverMessage }));

        // then
        expect(message).toBe(serverMessage);
    });
});

describe("participantRows", () => {
    it("lists facilitators first, then everyone by name, and marks the viewer", () => {
        // given
        const snapshot = makeBoardSnapshot({ participants: people });

        // when
        const rows = participantRows(snapshot);

        // then
        expect(rows.map(row => [row.participant.display_name, row.you])).toEqual([
            ["Bea", false],
            ["Sam", true],
            ["Ali", false],
            ["Zoe", false],
        ]);
    });

    it("lets a facilitator manage everyone but themselves", () => {
        // when
        const rows = participantRows(makeBoardSnapshot({ participants: people }));

        // then
        expect(rows.filter(row => row.manageable).map(row => row.participant.display_name)).toEqual([
            "Bea",
            "Ali",
            "Zoe",
        ]);
    });

    it("lets a participant manage nobody", () => {
        // when
        const rows = participantRows(makeBoardSnapshot({ participants: people, viewer: makeParticipantViewer() }));

        // then
        expect(rows.some(row => row.manageable)).toBe(false);
    });

    it("lets nobody manage anyone once the retro has ended", () => {
        // when
        const rows = participantRows(
            makeBoardSnapshot({ participants: people, board: makeBoardView({ closed: true }) }),
        );

        // then
        expect(rows.some(row => row.manageable)).toBe(false);
    });
});
