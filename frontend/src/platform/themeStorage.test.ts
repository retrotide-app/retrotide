import { afterEach, describe, expect, it, vi } from "vitest";
import type { ThemePreference } from "../types/app";
import { setPlatformErrorReporter } from "./errorReporter";
import { readThemePreference, THEME_STORAGE_KEY, writeThemePreference } from "./themeStorage";

afterEach(() => {
    vi.restoreAllMocks();
});

describe("THEME_STORAGE_KEY", () => {
    it("keeps the key a stored preference was saved under", () => {
        // given
        const expected = "rt-theme";

        // when
        const key = THEME_STORAGE_KEY;

        // then
        expect(key).toBe(expected);
    });
});

describe("readThemePreference", () => {
    const explicit: { name: string; preference: ThemePreference }[] = [
        { name: "reads back a stored light preference", preference: "light" },
        { name: "reads back a stored dark preference", preference: "dark" },
    ];

    it("finds no preference when nothing has been stored", () => {
        // given
        localStorage.removeItem(THEME_STORAGE_KEY);

        // when
        const preference = readThemePreference();

        // then
        expect(preference).toBeNull();
    });

    it.each(explicit)("$name", ({ preference }) => {
        // given
        writeThemePreference(preference);

        // when
        const stored = readThemePreference();

        // then
        expect(stored).toBe(preference);
    });

    it("finds no preference once system has replaced a stored one", () => {
        // given
        writeThemePreference("dark");
        writeThemePreference("system");

        // when
        const preference = readThemePreference();

        // then
        expect(preference).toBeNull();
    });

    it("ignores a stored value that is not a known preference", () => {
        // given
        localStorage.setItem(THEME_STORAGE_KEY, "sepia");

        // when
        const preference = readThemePreference();

        // then
        expect(preference).toBeNull();
    });

    it("reports a storage read that throws and finds no preference", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("site data is disabled", "SecurityError");
        vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const preference = readThemePreference();

        // then
        expect(preference).toBeNull();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});

describe("writeThemePreference", () => {
    const explicit: { name: string; preference: ThemePreference }[] = [
        { name: "stores a light preference under the theme key", preference: "light" },
        { name: "stores a dark preference under the theme key", preference: "dark" },
    ];

    it.each(explicit)("$name", ({ preference }) => {
        // given
        localStorage.removeItem(THEME_STORAGE_KEY);

        // when
        writeThemePreference(preference);

        // then
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe(preference);
    });

    it("removes the theme key when the preference is system", () => {
        // given
        localStorage.setItem(THEME_STORAGE_KEY, "dark");

        // when
        writeThemePreference("system");

        // then
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();
    });

    it("reports a storage write that throws and carries on", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("the quota is exhausted", "QuotaExceededError");
        vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const attempt = () => writeThemePreference("dark");

        // then
        expect(attempt).not.toThrow();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });

    it("reports a storage removal that throws and carries on", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("site data is disabled", "SecurityError");
        vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const attempt = () => writeThemePreference("system");

        // then
        expect(attempt).not.toThrow();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});
