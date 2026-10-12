import { describe, expect, it } from "vitest";
import { makeBoardSettings, makeBoardView } from "../../test-utils/fixtures";
import {
    backStep,
    BOARD_STAGES,
    nextStage,
    nextStep,
    previousStage,
    STAGE_INFO,
    STAGE_LABELS,
    totalsHidden,
} from "./stages";

describe("BOARD_STAGES", () => {
    it("runs the five stages in the order a retro moves through them", () => {
        // then
        expect(BOARD_STAGES).toEqual(["write", "vote", "discuss", "actions", "end"]);
        expect(STAGE_LABELS).toEqual(["Write", "Vote", "Discuss", "Actions", "End"]);
    });

    it("gives every stage one line on what it is for", () => {
        // then
        for (const stage of BOARD_STAGES) {
            expect(STAGE_INFO[stage].purpose).not.toBe("");
            expect(STAGE_INFO[stage].purpose).not.toContain("—");
        }
    });
});

describe("nextStage", () => {
    it.each([
        ["write", "vote"],
        ["vote", "discuss"],
        ["discuss", "actions"],
        ["actions", "end"],
    ] as const)("moves %s on to %s", (stage, next) => {
        // then
        expect(nextStage(stage)).toBe(next);
    });

    it("has nothing after End", () => {
        // then
        expect(nextStage("end")).toBeNull();
    });
});

describe("nextStep", () => {
    it("reveals the cards when Write moves on to Vote on a board that hides them", () => {
        // when
        const step = nextStep(makeBoardView({ stage: "write", cards_hidden: true }));

        // then
        expect(step).toEqual({ to: "vote", label: "Vote", revealsCards: true, endsRetro: false });
    });

    it("reveals nothing when the cards were already shown in Write", () => {
        // when
        const step = nextStep(makeBoardView({ stage: "write", cards_hidden: false }));

        // then
        expect(step?.revealsCards).toBe(false);
    });

    it("ends the retro when Actions moves on to End", () => {
        // when
        const step = nextStep(makeBoardView({ stage: "actions", cards_hidden: false }));

        // then
        expect(step).toEqual({ to: "end", label: "End", revealsCards: false, endsRetro: true });
    });

    it("needs no confirmation for the quiet steps in between", () => {
        // when
        const toDiscuss = nextStep(makeBoardView({ stage: "vote", cards_hidden: false }));
        const toActions = nextStep(makeBoardView({ stage: "discuss", cards_hidden: false }));

        // then
        expect(toDiscuss).toEqual({ to: "discuss", label: "Discuss", revealsCards: false, endsRetro: false });
        expect(toActions).toEqual({ to: "actions", label: "Actions", revealsCards: false, endsRetro: false });
    });

    it("offers no step on an ended board", () => {
        // then
        expect(nextStep(makeBoardView({ stage: "end", closed: true }))).toBeNull();
    });
});

describe("previousStage", () => {
    it.each([
        ["vote", "write"],
        ["discuss", "vote"],
        ["actions", "discuss"],
    ] as const)("steps %s back to %s", (stage, previous) => {
        // then
        expect(previousStage(stage)).toBe(previous);
    });

    it("has nothing before Write, and nothing back from End, which is final", () => {
        // then
        expect(previousStage("write")).toBeNull();
        expect(previousStage("end")).toBeNull();
    });
});

describe("backStep", () => {
    it("offers the stage before the board's", () => {
        // then
        expect(backStep(makeBoardView({ stage: "actions", cards_hidden: false }))).toBe("discuss");
    });

    it("offers no step back on a board in Write or an ended board", () => {
        // then
        expect(backStep(makeBoardView({ stage: "write" }))).toBeNull();
        expect(backStep(makeBoardView({ stage: "end", closed: true }))).toBeNull();
        expect(backStep(makeBoardView({ stage: "actions", closed: true }))).toBeNull();
    });
});

describe("totalsHidden", () => {
    it.each(["write", "vote"] as const)("hides totals in %s while the setting is on", stage => {
        // then
        expect(totalsHidden(makeBoardSettings({ vote_counts_hidden: true }), stage)).toBe(true);
    });

    it.each(["discuss", "actions", "end"] as const)("shows totals in %s even while the setting is on", stage => {
        // then
        expect(totalsHidden(makeBoardSettings({ vote_counts_hidden: true }), stage)).toBe(false);
    });

    it("shows totals in every stage while the setting is off", () => {
        // then
        for (const stage of BOARD_STAGES) {
            expect(totalsHidden(makeBoardSettings({ vote_counts_hidden: false }), stage)).toBe(false);
        }
    });
});
