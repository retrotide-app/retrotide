import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTheme } from "../hooks/useTheme";
import { renderWithProviders } from "../test-utils/render";
import type { ThemePreference } from "../types/app";
import { ThemeProvider } from "./ThemeContext";

const { readThemePreference, writeThemePreference } = vi.hoisted(() => ({
    readThemePreference: vi.fn<() => ThemePreference | null>(),
    writeThemePreference: vi.fn<(preference: ThemePreference) => void>(),
}));

vi.mock("../platform/themeStorage", () => ({ readThemePreference, writeThemePreference }));

function Probe() {
    const { preference, setPreference } = useTheme();

    return (
        <div>
            <p>{`preference: ${preference}`}</p>
            <button type="button" onClick={() => setPreference("light")}>
                choose light
            </button>
            <button type="button" onClick={() => setPreference("dark")}>
                choose dark
            </button>
            <button type="button" onClick={() => setPreference("system")}>
                choose system
            </button>
        </div>
    );
}

function renderProbe() {
    return renderWithProviders(
        <ThemeProvider>
            <Probe />
        </ThemeProvider>,
        { theme: { preference: "light" } },
    );
}

beforeEach(() => {
    readThemePreference.mockReset();
    readThemePreference.mockReturnValue(null);
    writeThemePreference.mockReset();
});

afterEach(() => {
    document.documentElement.removeAttribute("data-theme");
});

describe("ThemeProvider stored preference", () => {
    it("follows the system and leaves the document unmarked when nothing has been stored", () => {
        // given
        document.documentElement.setAttribute("data-theme", "dark");

        // when
        renderProbe();

        // then
        expect(screen.getByText("preference: system")).toBeInTheDocument();
        expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
    });

    it("marks the document dark when a dark preference has been stored", () => {
        // given
        readThemePreference.mockReturnValue("dark");

        // when
        renderProbe();

        // then
        expect(screen.getByText("preference: dark")).toBeInTheDocument();
        expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    });

    it("reads the stored preference without writing it straight back", () => {
        // given
        readThemePreference.mockReturnValue("dark");

        // when
        renderProbe();

        // then
        expect(readThemePreference).toHaveBeenCalledOnce();
        expect(writeThemePreference).not.toHaveBeenCalled();
    });
});

describe("ThemeProvider choosing a preference", () => {
    it("marks the document light and stores the choice when light is chosen", async () => {
        // given
        const user = userEvent.setup();
        renderProbe();

        // when
        await user.click(screen.getByRole("button", { name: "choose light" }));

        // then
        expect(screen.getByText("preference: light")).toBeInTheDocument();
        expect(document.documentElement.getAttribute("data-theme")).toBe("light");
        expect(writeThemePreference).toHaveBeenCalledExactlyOnceWith("light");
    });

    it("switches the document from light to dark when dark is chosen afterwards", async () => {
        // given
        readThemePreference.mockReturnValue("light");
        const user = userEvent.setup();
        renderProbe();

        // when
        await user.click(screen.getByRole("button", { name: "choose dark" }));

        // then
        expect(screen.getByText("preference: dark")).toBeInTheDocument();
        expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
        expect(writeThemePreference).toHaveBeenCalledExactlyOnceWith("dark");
    });

    it("unmarks the document and stores system when the system is chosen", async () => {
        // given
        readThemePreference.mockReturnValue("dark");
        const user = userEvent.setup();
        renderProbe();

        // when
        await user.click(screen.getByRole("button", { name: "choose system" }));

        // then
        expect(screen.getByText("preference: system")).toBeInTheDocument();
        expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
        expect(writeThemePreference).toHaveBeenCalledExactlyOnceWith("system");
    });
});
