import { describe, expect, it } from "vitest";
import {
    areaPath,
    COUNT_FORMAT,
    countOf,
    formatLongDay,
    formatShortDay,
    linePath,
    nearestIndex,
    niceTicks,
    peakPoint,
    pointPercent,
    seriesSummary,
    seriesTotal,
    valuePercent,
    weeklyLabelIndices,
} from "./chart";

const accounts = { one: "account", other: "accounts" };

describe("COUNT_FORMAT", () => {
    it("groups the thousands", () => {
        // when
        const shown = COUNT_FORMAT.format(48210);

        // then
        expect(shown).toBe("48,210");
    });
});

describe("countOf", () => {
    it("uses the singular for one", () => {
        // when
        const shown = countOf(1, accounts);

        // then
        expect(shown).toBe("1 account");
    });

    it("uses the plural for anything else", () => {
        // when
        const shown = countOf(0, accounts);

        // then
        expect(shown).toBe("0 accounts");
    });
});

describe("day formats", () => {
    it("names a day briefly in UTC", () => {
        // when
        const shown = formatShortDay("2026-09-12");

        // then
        expect(shown).toBe("12 Sept");
    });

    it("names a day in full in UTC", () => {
        // when
        const shown = formatLongDay("2026-09-12");

        // then
        expect(shown).toBe("Sat 12 September");
    });

    it("refuses a date it cannot read", () => {
        // when
        const read = () => formatShortDay("not a date");

        // then
        expect(read).toThrow('invalid chart date "not a date"');
    });
});

describe("niceTicks", () => {
    it("steps in round numbers up past the highest value", () => {
        // when
        const ticks = niceTicks(41);

        // then
        expect(ticks).toEqual([0, 20, 40, 60]);
    });

    it("never steps by less than one", () => {
        // when
        const ticks = niceTicks(3);

        // then
        expect(ticks).toEqual([0, 1, 2, 3]);
    });

    it("still draws a scale when every value is zero", () => {
        // when
        const ticks = niceTicks(0);

        // then
        expect(ticks).toEqual([0, 1]);
    });

    it("steps in hundreds for large values", () => {
        // when
        const ticks = niceTicks(902);

        // then
        expect(ticks).toEqual([0, 500, 1000]);
    });
});

describe("weeklyLabelIndices", () => {
    it("labels every seventh day counting back from the last", () => {
        // when
        const indices = weeklyLabelIndices(30);

        // then
        expect(indices).toEqual([1, 8, 15, 22, 29]);
    });
});

describe("series figures", () => {
    const points = [
        { date: "2026-10-03", value: 2 },
        { date: "2026-10-04", value: 5 },
        { date: "2026-10-05", value: 1 },
    ];

    it("adds up the series", () => {
        // when
        const total = seriesTotal(points);

        // then
        expect(total).toBe(8);
    });

    it("finds the busiest day", () => {
        // when
        const peak = peakPoint(points);

        // then
        expect(peak).toEqual({ date: "2026-10-04", value: 5 });
    });

    it("finds no busiest day in an empty series", () => {
        // when
        const peak = peakPoint([]);

        // then
        expect(peak).toBeNull();
    });

    it("sums up the period and its busiest day", () => {
        // when
        const summary = seriesSummary(points, accounts);

        // then
        expect(summary).toBe("8 accounts in the last 3 days. Busiest day: Sun 4 October, with 5.");
    });

    it("names no busiest day when nothing happened", () => {
        // given
        const quiet = points.map(point => ({ ...point, value: 0 }));

        // when
        const summary = seriesSummary(quiet, accounts);

        // then
        expect(summary).toBe("0 accounts in the last 3 days.");
    });
});

describe("geometry", () => {
    it("spreads the points from edge to edge", () => {
        // then
        expect(pointPercent(0, 3)).toBe(0);
        expect(pointPercent(1, 3)).toBe(50);
        expect(pointPercent(2, 3)).toBe(100);
        expect(pointPercent(0, 1)).toBe(50);
    });

    it("puts the top of the scale at the top of the plot", () => {
        // then
        expect(valuePercent(0, 10)).toBe(100);
        expect(valuePercent(10, 10)).toBe(0);
        expect(valuePercent(5, 10)).toBe(50);
    });

    it("draws the line through every point", () => {
        // when
        const path = linePath(
            [
                { date: "2026-10-04", value: 0 },
                { date: "2026-10-05", value: 10 },
            ],
            10,
        );

        // then
        expect(path).toBe("M0 100 L100 0");
    });

    it("closes the area along the baseline", () => {
        // when
        const path = areaPath(
            [
                { date: "2026-10-04", value: 0 },
                { date: "2026-10-05", value: 10 },
            ],
            10,
        );

        // then
        expect(path).toBe("M0 100 L100 0 L100 100 L0 100 Z");
    });

    it("draws no area without points", () => {
        // when
        const path = areaPath([], 10);

        // then
        expect(path).toBe("");
    });

    it("snaps the pointer to the nearest day", () => {
        // then
        expect(nearestIndex(0.49, 3)).toBe(1);
        expect(nearestIndex(-0.2, 3)).toBe(0);
        expect(nearestIndex(1.4, 3)).toBe(2);
        expect(nearestIndex(0.7, 1)).toBe(0);
    });
});
