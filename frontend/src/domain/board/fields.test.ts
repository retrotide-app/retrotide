import { describe, expect, it } from "vitest";
import { BOARD_COLOURS, COLOUR_LABELS, DEFAULT_COLUMN_COLOUR, voteLimitFrom } from "./fields";

describe("board colours", () => {
    it("offers the eight board colours in a fixed order, each with a label", () => {
        // when
        const labels = BOARD_COLOURS.map(colour => COLOUR_LABELS[colour]);

        // then
        expect(labels).toEqual(["Green", "Red", "Blue", "Amber", "Purple", "Teal", "Pink", "Grey"]);
    });

    it("gives a new column the grey the server uses when none is chosen", () => {
        // then
        expect(DEFAULT_COLUMN_COLOUR).toBe("grey");
    });
});

describe("voteLimitFrom", () => {
    it("reads a whole number from 0 to 99", () => {
        // then
        expect(["0", " 6 ", "99"].map(voteLimitFrom)).toEqual([0, 6, 99]);
    });

    it("refuses anything else", () => {
        // then
        expect(["", "100", "-1", "2.5", "six", "1e2"].map(voteLimitFrom)).toEqual([null, null, null, null, null, null]);
    });
});
