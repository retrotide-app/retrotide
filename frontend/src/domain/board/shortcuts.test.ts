import { describe, expect, it } from "vitest";
import { SHORTCUTS, type ShortcutKey, shortcutFor } from "./shortcuts";

function press(overrides: Partial<ShortcutKey>): ShortcutKey {
    return { key: "", ctrlKey: false, metaKey: false, altKey: false, targetTag: "BODY", editable: false, ...overrides };
}

describe("shortcutFor", () => {
    it.each([
        ["?", "help"],
        ["n", "new-card"],
        ["/", "search"],
        ["l", "layout"],
    ] as const)("maps %s to %s", (key, action) => {
        // when
        const got = shortcutFor(press({ key }));

        // then
        expect(got).toBe(action);
    });

    it.each(["x", "Escape", "Enter", "N", "L", "constructor", ""])("ignores %j", key => {
        // when
        const got = shortcutFor(press({ key }));

        // then
        expect(got).toBeNull();
    });

    it.each([
        ["ctrl", { ctrlKey: true }],
        ["meta", { metaKey: true }],
        ["alt", { altKey: true }],
    ] as const)("ignores a key pressed with %s held", (_name, modifier) => {
        // when
        const got = shortcutFor(press({ key: "n", ...modifier }));

        // then
        expect(got).toBeNull();
    });

    it.each(["INPUT", "textarea", "SELECT"])("ignores keys typed into a %s", targetTag => {
        // when
        const got = shortcutFor(press({ key: "/", targetTag }));

        // then
        expect(got).toBeNull();
    });

    it("ignores keys typed into an editable element", () => {
        // when
        const got = shortcutFor(press({ key: "l", targetTag: "DIV", editable: true }));

        // then
        expect(got).toBeNull();
    });
});

describe("SHORTCUTS", () => {
    it("lists every shortcut once, Escape included", () => {
        // when
        const keys = SHORTCUTS.map(entry => entry.keys);

        // then
        expect(keys).toEqual(["?", "N", "/", "L", "Esc"]);
    });
});
