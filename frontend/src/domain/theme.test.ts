import { describe, expect, it } from "vitest";
import { isThemePreference, THEME_PREFERENCES } from "./theme";

describe("THEME_PREFERENCES", () => {
    it("offers system, light and dark, in that order", () => {
        // given
        const expected = ["system", "light", "dark"];

        // when
        const offered = [...THEME_PREFERENCES];

        // then
        expect(offered).toEqual(expected);
    });
});

describe("isThemePreference", () => {
    const accepted: { name: string; value: string }[] = [
        { name: "follows the system when asked to", value: "system" },
        { name: "accepts the light theme", value: "light" },
        { name: "accepts the dark theme", value: "dark" },
    ];

    const refused: { name: string; value: string }[] = [
        { name: "refuses an unknown preference", value: "sepia" },
        { name: "refuses an empty string", value: "" },
        { name: "refuses a capitalised preference", value: "Dark" },
        { name: "refuses an upper case preference", value: "LIGHT" },
        { name: "refuses a mixed case preference", value: "sYsTeM" },
    ];

    it.each(accepted)("$name", ({ value }) => {
        // given
        const stored = value;

        // when
        const result = isThemePreference(stored);

        // then
        expect(result).toBe(true);
    });

    it.each(refused)("$name", ({ value }) => {
        // given
        const stored = value;

        // when
        const result = isThemePreference(stored);

        // then
        expect(result).toBe(false);
    });
});
