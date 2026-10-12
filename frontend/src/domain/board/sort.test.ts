import { describe, expect, it } from "vitest";
import { compareByPosition, compareBytes } from "./sort";

describe("compareBytes", () => {
    it("orders by character code, so capitals and digits come before small letters", () => {
        // given
        const keys = ["a", "Z", "0", "b", "A"];

        // when
        const sorted = keys.toSorted(compareBytes);

        // then
        expect(sorted).toEqual(["0", "A", "Z", "a", "b"]);
    });

    it("puts a key before every longer key it starts", () => {
        // when
        const order = [compareBytes("a", "a0"), compareBytes("a0", "a"), compareBytes("a0", "a0")];

        // then
        expect(Math.sign(order[0])).toBe(-1);
        expect(Math.sign(order[1])).toBe(1);
        expect(order[2]).toBe(0);
    });

    it("orders by code point as UTF-8 bytes do, not by UTF-16 code unit", () => {
        // given
        const lastBasic = "\u{FFFF}";
        const firstAstral = "\u{10000}";

        // when
        const order = compareBytes(lastBasic, firstAstral);

        // then
        expect(Math.sign(order)).toBe(-1);
        expect(lastBasic < firstAstral).toBe(false);
    });
});

describe("compareByPosition", () => {
    it("orders by position first", () => {
        // given
        const items = [
            { id: "a", position: "a2" },
            { id: "b", position: "a0" },
            { id: "c", position: "a1" },
        ];

        // when
        const sorted = items.toSorted(compareByPosition);

        // then
        expect(sorted.map(item => item.id)).toEqual(["b", "c", "a"]);
    });

    it("breaks a tie on position by id, so every viewer sees the same order", () => {
        // given
        const items = [
            { id: "f2", position: "a0" },
            { id: "E9", position: "a0" },
            { id: "f1", position: "a0" },
        ];

        // when
        const sorted = items.toSorted(compareByPosition);

        // then
        expect(sorted.map(item => item.id)).toEqual(["E9", "f1", "f2"]);
    });
});
