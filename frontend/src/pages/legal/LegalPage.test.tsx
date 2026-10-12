import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { LegalPage } from "./LegalPage";

const LAST_UPDATED = "Last updated October 2026";

const DOCUMENTS = [
    { page: "privacy", heading: "Privacy notice" },
    { page: "terms", heading: "Terms of service" },
    { page: "acceptable-use", heading: "Acceptable use policy" },
    { page: "cookies", heading: "Cookie statement" },
    { page: "sub-processors", heading: "Sub-processors" },
    { page: "security", heading: "Security" },
];

function renderLegalPage(page: string) {
    return renderWithProviders(<LegalPage />, { route: `/legal/${page}`, path: "/legal/:page" });
}

beforeEach(() => {
    document.title = "";
});

describe("LegalPage", () => {
    it.each(DOCUMENTS)("renders the $page document under its heading", ({ page, heading }) => {
        // given
        const route = page;

        // when
        renderLegalPage(route);

        // then
        expect(screen.getByRole("heading", { level: 1, name: heading })).toBeInTheDocument();
    });

    it.each(DOCUMENTS)("dates the $page document", ({ page }) => {
        // given
        const route = page;

        // when
        renderLegalPage(route);

        // then
        expect(screen.getByText(LAST_UPDATED)).toBeInTheDocument();
    });

    it.each(DOCUMENTS)("names the $page document in the browser tab", ({ page, heading }) => {
        // given
        const route = page;

        // when
        renderLegalPage(route);

        // then
        expect(document.title).toContain(heading);
        expect(document.title).toContain("RetroTide");
    });

    it("draws the document inside the marketing layout", () => {
        // given
        const route = "privacy";

        // when
        renderLegalPage(route);

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { level: 1, name: "Privacy notice" }),
        );
        expect(screen.getByRole("contentinfo")).toBeInTheDocument();
    });

    it("shows the not-found page for an unknown legal page", () => {
        // given
        const route = "nonsense";

        // when
        renderLegalPage(route);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Page not found" })).toBeInTheDocument();
        expect(screen.queryByText(LAST_UPDATED)).not.toBeInTheDocument();
        expect(document.title).toContain("Page not found");
    });

    it("never treats an inherited object key as a legal page", () => {
        // given
        const route = "constructor";

        // when
        renderLegalPage(route);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Page not found" })).toBeInTheDocument();
    });
});
