import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardTemplate, makeQueryState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { TemplateListResponse } from "../../types/api";
import { TemplatesPage } from "./TemplatesPage";

const { useBoardTemplates } = vi.hoisted(() => ({ useBoardTemplates: vi.fn() }));

vi.mock("../../hooks/queries/boards", () => ({ useBoardTemplates }));

const sailboat = makeBoardTemplate({
    key: "sailboat",
    name: "Sailboat",
    description: "Look at what drives the team and what holds it back.",
    columns: [
        { title: "Wind", description: "What pushed us forward?", colour: "green" },
        { title: "Anchors", description: "What held us back?", colour: "red" },
        { title: "Rocks", description: "What risks lie ahead?", colour: "amber" },
        { title: "Island", description: "Where are we heading?", colour: "blue" },
    ],
});

const madSadGlad = makeBoardTemplate({
    key: "mad-sad-glad",
    name: "Mad, Sad, Glad",
    description: "Talk about how the sprint felt.",
    columns: [
        { title: "Mad", description: "What frustrated you?", colour: "red" },
        { title: "Sad", description: "What disappointed you?", colour: "blue" },
        { title: "Glad", description: "What made you happy?", colour: "green" },
    ],
});

const blank = makeBoardTemplate();

function showCatalogue(templates = [sailboat, madSadGlad, blank]) {
    useBoardTemplates.mockReturnValue(makeQueryState<TemplateListResponse>({ data: { templates } }));
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showCatalogue();
});

describe("TemplatesPage", () => {
    it("heads the page and titles the document", () => {
        // when
        renderWithProviders(<TemplatesPage />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Retrospective templates" })).toBeInTheDocument();
        expect(document.title).toContain("Retrospective templates");
    });

    it("lists every template but blank, each linking to its own page", () => {
        // when
        renderWithProviders(<TemplatesPage />);

        // then
        const list = within(screen.getByRole("list", { name: "Templates" }));
        expect(list.getAllByRole("heading", { level: 3 }).map(heading => heading.textContent)).toEqual([
            "Sailboat",
            "Mad, Sad, Glad",
        ]);
        expect(list.getByRole("link", { name: "Sailboat" })).toHaveAttribute("href", "/templates/sailboat");
        expect(list.getByRole("link", { name: "Mad, Sad, Glad" })).toHaveAttribute("href", "/templates/mad-sad-glad");
        expect(list.queryByRole("link", { name: "Blank" })).not.toBeInTheDocument();
    });

    it("shows each template's description and column count", () => {
        // when
        renderWithProviders(<TemplatesPage />);

        // then
        expect(screen.getByText("Look at what drives the team and what holds it back.")).toBeInTheDocument();
        expect(screen.getByText("4 columns")).toBeInTheDocument();
        expect(screen.getByText("3 columns")).toBeInTheDocument();
    });

    it("says it is loading while the catalogue is on its way", () => {
        // given
        useBoardTemplates.mockReturnValue(makeQueryState<TemplateListResponse>({ loading: true }));

        // when
        renderWithProviders(<TemplatesPage />);

        // then
        expect(screen.getByText("Loading templates...")).toBeInTheDocument();
        expect(screen.queryByRole("list", { name: "Templates" })).not.toBeInTheDocument();
    });

    it("shows the server's reason when the catalogue could not load, and tries again", async () => {
        // given
        const state = makeQueryState<TemplateListResponse>({
            error: makeApiError({ status: 500, message: "internal error" }),
        });
        useBoardTemplates.mockReturnValue(state);
        const user = userEvent.setup();
        renderWithProviders(<TemplatesPage />);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("internal error");
        expect(state.refetch).toHaveBeenCalledOnce();
    });
});
