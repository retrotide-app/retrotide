import { describe, expect, it } from "vitest";
import { makeTimer } from "../../test-utils/fixtures";
import {
    canAddMinute,
    formatTimer,
    readTimer,
    timerLengthLabel,
    timerLengthMs,
    timerRemainingMs,
    timerTone,
} from "./timer";

const ENDS_AT = Date.parse("2026-10-01T09:35:00.000Z");

describe("timerRemainingMs", () => {
    it("counts down to the end on the server's clock", () => {
        // given
        const timer = makeTimer();

        // then
        expect(timerRemainingMs(timer, ENDS_AT - 90_000)).toBe(90_000);
    });

    it("never goes below zero once the end has passed", () => {
        // then
        expect(timerRemainingMs(makeTimer(), ENDS_AT + 5_000)).toBe(0);
    });

    it("holds what was left while the timer is paused, whatever the clock says", () => {
        // given
        const paused = makeTimer({ ends_at: null, paused_remaining_ms: 42_000 });

        // then
        expect(timerRemainingMs(paused, ENDS_AT + 600_000)).toBe(42_000);
    });
});

describe("timerTone", () => {
    it("stays calm until the last minute, turns amber, then red in the last fifteen seconds", () => {
        // given
        const fiveMinutes = 300_000;

        // then
        expect(timerTone(61_000, fiveMinutes)).toBe("calm");
        expect(timerTone(60_000, fiveMinutes)).toBe("amber");
        expect(timerTone(15_001, fiveMinutes)).toBe("amber");
        expect(timerTone(15_000, fiveMinutes)).toBe("red");
        expect(timerTone(0, fiveMinutes)).toBe("red");
    });

    it("scales the warnings down for a short timer, so a one minute timer is not amber from the start", () => {
        // given
        const oneMinute = 60_000;

        // then
        expect(timerTone(60_000, oneMinute)).toBe("calm");
        expect(timerTone(15_000, oneMinute)).toBe("amber");
        expect(timerTone(6_000, oneMinute)).toBe("red");
    });
});

describe("readTimer", () => {
    it("reads a running timer", () => {
        // then
        expect(readTimer(makeTimer(), ENDS_AT - 120_000)).toEqual({
            remainingMs: 120_000,
            phase: "running",
            tone: "calm",
        });
    });

    it("reads a paused timer", () => {
        // given
        const paused = makeTimer({ ends_at: null, paused_remaining_ms: 10_000 });

        // then
        expect(readTimer(paused, ENDS_AT)).toEqual({ remainingMs: 10_000, phase: "paused", tone: "red" });
    });

    it("reads a timer that has run out as finished", () => {
        // then
        expect(readTimer(makeTimer(), ENDS_AT + 1)).toEqual({ remainingMs: 0, phase: "finished", tone: "red" });
    });
});

describe("formatTimer", () => {
    it("shows minutes and seconds, rounding a part second up so the end reads 0:00 only at the end", () => {
        // then
        expect(formatTimer(300_000)).toBe("5:00");
        expect(formatTimer(61_000)).toBe("1:01");
        expect(formatTimer(59_001)).toBe("1:00");
        expect(formatTimer(999)).toBe("0:01");
        expect(formatTimer(0)).toBe("0:00");
        expect(formatTimer(3_600_000)).toBe("60:00");
    });
});

describe("canAddMinute", () => {
    it("allows a minute while the timer stays within an hour", () => {
        // given
        const timer = makeTimer({ duration_ms: 3_540_000 });

        // then
        expect(canAddMinute(timer, readTimer(timer, ENDS_AT - 1_000))).toBe(true);
    });

    it("refuses a minute that would take a running timer past an hour", () => {
        // given
        const timer = makeTimer({ duration_ms: 3_570_000 });

        // then
        expect(canAddMinute(timer, readTimer(timer, ENDS_AT - 1_000))).toBe(false);
    });

    it("always allows a minute once the timer has run out, because it starts a fresh minute", () => {
        // given
        const timer = makeTimer({ duration_ms: 3_600_000 });

        // then
        expect(canAddMinute(timer, readTimer(timer, ENDS_AT + 1_000))).toBe(true);
    });
});

describe("timer lengths", () => {
    it("turns minutes into milliseconds and names them", () => {
        // then
        expect(timerLengthMs(5)).toBe(300_000);
        expect(timerLengthLabel(1)).toBe("1 minute");
        expect(timerLengthLabel(10)).toBe("10 minutes");
    });
});
