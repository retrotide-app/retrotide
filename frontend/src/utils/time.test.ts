import { beforeEach, describe, expect, it, vi } from "vitest";
import { formatDate, formatDuration, formatExactDateTime, parseServerDate, relativeTime } from "./time";

const NOW = new Date(2026, 5, 15, 12, 0, 0).getTime();

function ago(seconds: number): string {
    return new Date(NOW - seconds * 1000).toISOString();
}

describe("parseServerDate", () => {
    it("parses an ISO timestamp that already carries a zone", () => {
        // given
        const input = "2026-06-15T12:00:00Z";

        // when
        const parsed = parseServerDate(input);

        // then
        expect(parsed?.toISOString()).toBe("2026-06-15T12:00:00.000Z");
    });

    it("treats a space separated server timestamp without a zone as UTC", () => {
        // given
        const input = "2026-06-15 12:00:00";

        // when
        const parsed = parseServerDate(input);

        // then
        expect(parsed?.toISOString()).toBe("2026-06-15T12:00:00.000Z");
    });

    it("honours an explicit offset with or without a colon", () => {
        // given
        const withColon = "2026-06-15T14:00:00+02:00";
        const withoutColon = "2026-06-15T14:00:00+0200";

        // when
        const a = parseServerDate(withColon);
        const b = parseServerDate(withoutColon);

        // then
        expect(a?.toISOString()).toBe("2026-06-15T12:00:00.000Z");
        expect(b?.toISOString()).toBe("2026-06-15T12:00:00.000Z");
    });

    it("keeps sub second precision from a postgres style timestamp", () => {
        // given
        const input = "2026-06-15 12:00:00.123456+00:00";

        // when
        const parsed = parseServerDate(input);

        // then
        expect(parsed?.toISOString()).toBe("2026-06-15T12:00:00.123Z");
    });

    it("trims surrounding whitespace before parsing", () => {
        // given
        const input = "  2026-06-15T12:00:00Z  ";

        // when
        const parsed = parseServerDate(input);

        // then
        expect(parsed?.toISOString()).toBe("2026-06-15T12:00:00.000Z");
    });

    it("returns null for missing or blank input", () => {
        // given
        const inputs: (string | null | undefined)[] = [null, undefined, "", "   "];

        // when
        const parsed = inputs.map(input => parseServerDate(input));

        // then
        expect(parsed).toEqual([null, null, null, null]);
    });

    it("returns null for a string that is not a date", () => {
        // given
        const inputs = ["not a date", "2026-13-45T99:00:00Z"];

        // when
        const parsed = inputs.map(input => parseServerDate(input));

        // then
        expect(parsed).toEqual([null, null]);
    });
});

describe("relativeTime", () => {
    beforeEach(() => {
        vi.useFakeTimers();
        vi.setSystemTime(NOW);
    });

    it("returns an empty string when there is no date", () => {
        // given
        const stamps: (string | null | undefined)[] = [null, undefined, "", "nonsense"];

        // when
        const results = stamps.map(stamp => relativeTime(stamp));

        // then
        expect(results).toEqual(["", "", "", ""]);
    });

    it("calls anything under a minute just now", () => {
        // given
        const stamps = [ago(0), ago(59)];

        // when
        const results = stamps.map(stamp => relativeTime(stamp));

        // then
        expect(results).toEqual(["just now", "just now"]);
    });

    it("switches to minutes at the one minute boundary", () => {
        // given
        const stamps = [ago(60), ago(59 * 60 + 59)];

        // when
        const results = stamps.map(stamp => relativeTime(stamp));

        // then
        expect(results).toEqual(["1m ago", "59m ago"]);
    });

    it("switches to hours at the one hour boundary", () => {
        // given
        const stamps = [ago(60 * 60), ago(24 * 60 * 60 - 1)];

        // when
        const results = stamps.map(stamp => relativeTime(stamp));

        // then
        expect(results).toEqual(["1h ago", "23h ago"]);
    });

    it("switches to days at the one day boundary", () => {
        // given
        const stamps = [ago(24 * 60 * 60), ago(30 * 24 * 60 * 60 - 1)];

        // when
        const results = stamps.map(stamp => relativeTime(stamp));

        // then
        expect(results).toEqual(["1d ago", "29d ago"]);
    });

    it("switches to thirty day months at the thirty day boundary", () => {
        // given
        const stamps = [ago(30 * 24 * 60 * 60), ago(59 * 24 * 60 * 60), ago(60 * 24 * 60 * 60)];

        // when
        const results = stamps.map(stamp => relativeTime(stamp));

        // then
        expect(results).toEqual(["1mo ago", "1mo ago", "2mo ago"]);
    });

    it("keeps counting in months rather than rolling over into years", () => {
        // given
        const twoYears = ago(730 * 24 * 60 * 60);

        // when
        const result = relativeTime(twoYears);

        // then
        expect(result).toBe("24mo ago");
    });

    it("treats a future timestamp as just now", () => {
        // given
        const future = ago(-3600);

        // when
        const result = relativeTime(future);

        // then
        expect(result).toBe("just now");
    });

    it("reads a server timestamp without a zone as UTC", () => {
        // given
        const zoned = ago(30 * 60);
        const zoneless = zoned.replace("T", " ").replace("Z", "");

        // when
        const result = relativeTime(zoneless);

        // then
        expect(result).toBe("30m ago");
    });

    it("uses the long style unless told otherwise", () => {
        // given
        const stamp = ago(2 * 60 * 60);

        // when
        const implicit = relativeTime(stamp);
        const explicit = relativeTime(stamp, "long");

        // then
        expect(implicit).toBe("2h ago");
        expect(explicit).toBe("2h ago");
    });

    describe("in the short style", () => {
        it("returns an empty string when there is no date", () => {
            // given
            const stamps: (string | null | undefined)[] = [null, ""];

            // when
            const results = stamps.map(stamp => relativeTime(stamp, "short"));

            // then
            expect(results).toEqual(["", ""]);
        });

        it("drops the ago suffix from every unit but keeps just now", () => {
            // given
            const stamps = [ago(5), ago(60), ago(2 * 60 * 60), ago(3 * 24 * 60 * 60), ago(90 * 24 * 60 * 60)];

            // when
            const results = stamps.map(stamp => relativeTime(stamp, "short"));

            // then
            expect(results).toEqual(["just now", "1m", "2h", "3d", "3mo"]);
        });

        it("uses the same boundaries as the long form", () => {
            // given
            const stamps = [ago(59), ago(59 * 60 + 59), ago(24 * 60 * 60 - 1), ago(30 * 24 * 60 * 60 - 1)];

            // when
            const results = stamps.map(stamp => relativeTime(stamp, "short"));

            // then
            expect(results).toEqual(["just now", "59m", "23h", "29d"]);
        });
    });
});

describe("formatDate", () => {
    it("returns an empty string when there is no date", () => {
        // given
        const stamps: (string | null | undefined)[] = [null, undefined, "", "not a date"];

        // when
        const results = stamps.map(stamp => formatDate(stamp));

        // then
        expect(results).toEqual(["", "", "", ""]);
    });

    it("renders a British date when no locale is given", () => {
        // given
        const local = new Date(2026, 0, 15, 12, 0, 0);

        // when
        const result = formatDate(local.toISOString());

        // then
        expect(result).toBe("15/01/2026");
    });

    it("renders day first for a British locale and month first for an American one", () => {
        // given
        const local = new Date(2026, 0, 15, 12, 0, 0);

        // when
        const gb = formatDate(local.toISOString(), "en-GB");
        const us = formatDate(local.toISOString(), "en-US");

        // then
        expect(gb).toBe("15/01/2026");
        expect(us).toBe("1/15/2026");
    });

    it("accepts a list of locales and uses the first", () => {
        // given
        const local = new Date(2026, 0, 15, 12, 0, 0);

        // when
        const result = formatDate(local.toISOString(), ["en-US", "en-GB"]);

        // then
        expect(result).toBe("1/15/2026");
    });
});

describe("formatExactDateTime", () => {
    const britishCases: { name: string; local: Date; expected: string }[] = [
        {
            name: "spells the month out and keeps a 24 hour clock",
            local: new Date(2026, 0, 15, 14, 30, 0),
            expected: "15 January 2026 at 14:30",
        },
        {
            name: "pads a single digit hour to two digits",
            local: new Date(2026, 0, 15, 9, 5, 0),
            expected: "15 January 2026 at 09:05",
        },
        {
            name: "renders midnight as the start of the day",
            local: new Date(2026, 0, 15, 0, 0, 0),
            expected: "15 January 2026 at 00:00",
        },
        {
            name: "renders the last minute of the day without rolling over",
            local: new Date(2026, 0, 15, 23, 59, 0),
            expected: "15 January 2026 at 23:59",
        },
        {
            name: "leaves a single digit day unpadded",
            local: new Date(2026, 2, 5, 8, 0, 0),
            expected: "5 March 2026 at 08:00",
        },
        {
            name: "ignores the seconds",
            local: new Date(2026, 11, 1, 6, 7, 42),
            expected: "1 December 2026 at 06:07",
        },
    ];

    const localeCases: { name: string; locale: string; expected: string }[] = [
        {
            name: "puts the day before the month for a British reader",
            locale: "en-GB",
            expected: "15 January 2026 at 14:30",
        },
        {
            name: "translates the month and the joining word for a French reader",
            locale: "fr-FR",
            expected: "15 janvier 2026 à 14:30",
        },
        {
            name: "translates the month and the joining word for a German reader",
            locale: "de-DE",
            expected: "15. Januar 2026 um 14:30",
        },
    ];

    it("returns an empty string when there is no date", () => {
        // given
        const stamps: (string | null | undefined)[] = [null, undefined, "", "   ", "not a date"];

        // when
        const results = stamps.map(stamp => formatExactDateTime(stamp));

        // then
        expect(results).toEqual(["", "", "", "", ""]);
    });

    it.each(britishCases)("$name", ({ local, expected }) => {
        // given
        const stamp = local.toISOString();

        // when
        const result = formatExactDateTime(stamp, "en-GB");

        // then
        expect(result).toBe(expected);
    });

    it.each(localeCases)("$name", ({ locale, expected }) => {
        // given
        const stamp = new Date(2026, 0, 15, 14, 30, 0).toISOString();

        // when
        const result = formatExactDateTime(stamp, locale);

        // then
        expect(result).toBe(expected);
    });

    it("switches an American reader to a twelve hour clock with the month first", () => {
        // given
        const stamp = new Date(2026, 0, 15, 14, 30, 0).toISOString();

        // when
        const result = formatExactDateTime(stamp, "en-US");

        // then
        expect(result).toMatch(/^January 15, 2026 at 2:30\sPM$/);
    });

    it("uses the British format when no locale is given", () => {
        // given
        const stamp = new Date(2026, 0, 15, 14, 30, 0).toISOString();

        // when
        const result = formatExactDateTime(stamp);

        // then
        expect(result).toBe("15 January 2026 at 14:30");
    });
});

describe("formatDuration", () => {
    it("names every level that has a value, largest first", () => {
        // given
        const ms = ((366 * 24 + 3) * 3600 + 4 * 60 + 5) * 1000;

        // when
        const result = formatDuration(ms);

        // then
        expect(result).toBe("1 year 1 day 3 hours 4 minutes 5 seconds");
    });

    it("skips the levels that are empty", () => {
        // given
        const ms = (2 * 3600 + 30) * 1000;

        // when
        const result = formatDuration(ms);

        // then
        expect(result).toBe("2 hours 30 seconds");
    });

    it("drops the plural for a single unit", () => {
        // given
        const ms = 60000;

        // when
        const result = formatDuration(ms);

        // then
        expect(result).toBe("1 minute");
    });

    it("says zero seconds rather than nothing at all", () => {
        // given
        const ms = 0;

        // when
        const result = formatDuration(ms);

        // then
        expect(result).toBe("0 seconds");
    });

    it("rounds a part second to the nearest second", () => {
        // given
        const ms = 1600;

        // when
        const result = formatDuration(ms);

        // then
        expect(result).toBe("2 seconds");
    });
});
