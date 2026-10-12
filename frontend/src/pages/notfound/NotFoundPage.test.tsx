import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { NotFoundPage } from "./NotFoundPage";

beforeEach(() => {
    document.title = "";
});

describe("NotFoundPage", () => {
    it("announces the missing page", () => {
        // given
        const ui = <NotFoundPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("404")).toBeInTheDocument();
        expect(screen.getByRole("heading", { level: 1, name: "Page not found" })).toBeInTheDocument();
    });

    it("explains why the page could not be found", () => {
        // given
        const ui = <NotFoundPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText(/does not exist/)).toBeInTheDocument();
        expect(screen.getByText(/mistyped/)).toBeInTheDocument();
    });

    it("offers the way back to the home page", () => {
        // given
        const ui = <NotFoundPage />;

        // when
        renderWithProviders(ui, { route: "/nowhere" });

        // then
        expect(screen.getByRole("link", { name: "Back to the home page" })).toHaveAttribute("href", "/");
    });

    it("tells the browser tab the page was not found", () => {
        // given
        const ui = <NotFoundPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(document.title).toContain("Page not found");
        expect(document.title).toContain("RetroTide");
    });
});
