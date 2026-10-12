import { describe, expect, it } from "vitest";
import { LATENCY_SAMPLES, clockOffsetSample, keepLatestSamples, latencyGrade, medianOf } from "./clock";

describe("latencyGrade", () => {
    it("grades a round trip as good, fair or poor", () => {
        // then
        expect(latencyGrade(40)).toBe("good");
        expect(latencyGrade(89)).toBe("good");
        expect(latencyGrade(90)).toBe("fair");
        expect(latencyGrade(169)).toBe("fair");
        expect(latencyGrade(170)).toBe("poor");
    });
});

describe("medianOf", () => {
    it("has no median without samples", () => {
        // then
        expect(medianOf([])).toBeNull();
    });

    it("takes the middle sample of an odd count and the mean of the middle two of an even count", () => {
        // then
        expect(medianOf([900, 200, 210])).toBe(210);
        expect(medianOf([100, 300, 200, 400])).toBe(250);
    });

    it("never reorders the samples it was given", () => {
        // given
        const samples = [3, 1, 2];

        // when
        medianOf(samples);

        // then
        expect(samples).toEqual([3, 1, 2]);
    });
});

describe("keepLatestSamples", () => {
    it("adds the new sample and keeps only the latest few", () => {
        // given
        const full = Array.from({ length: LATENCY_SAMPLES }, (_, index) => index);

        // when
        const kept = keepLatestSamples(full, 99);

        // then
        expect(kept).toHaveLength(LATENCY_SAMPLES);
        expect(kept.at(-1)).toBe(99);
        expect(kept[0]).toBe(1);
    });
});

describe("clockOffsetSample", () => {
    it("works out how far the server's clock is ahead, allowing half the round trip for the reply", () => {
        // given
        const serverNow = "2026-09-22T10:06:02.512Z";
        const receivedAt = Date.parse(serverNow) - 1_000;

        // when
        const offset = clockOffsetSample(serverNow, receivedAt, 100);

        // then
        expect(offset).toBe(1_050);
    });

    it("gives a negative offset when the browser's clock is ahead", () => {
        // given
        const serverNow = "2026-09-22T10:06:02.512Z";

        // when
        const offset = clockOffsetSample(serverNow, Date.parse(serverNow) + 500, 0);

        // then
        expect(offset).toBe(-500);
    });

    it("has no sample when the server's clock does not parse", () => {
        // then
        expect(clockOffsetSample("yesterday", 0, 10)).toBeNull();
    });
});
