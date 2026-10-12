import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { ThemeSelect } from "./ThemeSelect";

let windowErrors: unknown[] = [];

function captureWindowError(event: ErrorEvent): void {
    event.preventDefault();
    windowErrors.push(event.error);
}

describe("ThemeSelect", () => {
    it("is named by its visible Theme label", () => {
        // given
        const ui = <ThemeSelect />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Theme")).toBeVisible();
        expect(screen.getByLabelText("Theme")).toBeInstanceOf(HTMLSelectElement);
    });

    it("offers the system, light and dark preferences in that order", () => {
        // given
        const ui = <ThemeSelect />;

        // when
        renderWithProviders(ui);

        // then
        const options = screen.getAllByRole<HTMLOptionElement>("option");
        expect(options.map(option => option.textContent)).toEqual(["System", "Light", "Dark"]);
        expect(options.map(option => option.value)).toEqual(["system", "light", "dark"]);
    });

    it("shows the preference in use as the current selection", () => {
        // given
        const theme = { preference: "light" as const };

        // when
        renderWithProviders(<ThemeSelect />, { theme });

        // then
        expect(screen.getByRole("combobox", { name: "Theme" })).toHaveValue("light");
        expect(screen.getByRole<HTMLOptionElement>("option", { name: "Light" }).selected).toBe(true);
        expect(screen.getByRole<HTMLOptionElement>("option", { name: "System" }).selected).toBe(false);
    });

    it("hands the chosen preference to the theme context", async () => {
        // given
        const setPreference = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ThemeSelect />, { theme: { setPreference } });

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Theme" }), "Dark");

        // then
        expect(setPreference).toHaveBeenCalledExactlyOnceWith("dark");
    });
});

describe("ThemeSelect guarding its preferences", () => {
    beforeEach(() => {
        windowErrors = [];
        window.addEventListener("error", captureWindowError);
    });

    afterEach(() => {
        window.removeEventListener("error", captureWindowError);
    });

    it("throws on a value that is not a theme preference instead of ignoring it", async () => {
        // given
        const setPreference = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ThemeSelect />, { theme: { setPreference } });
        const select = screen.getByRole("combobox", { name: "Theme" });
        select.append(new Option("Sepia", "sepia"));

        // when
        await user.selectOptions(select, "sepia");

        // then
        expect(windowErrors).toEqual([new Error('unknown theme preference "sepia"')]);
        expect(setPreference).not.toHaveBeenCalled();
    });
});
