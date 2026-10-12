import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardTemplate,
    makeOrganisation,
    makeQueryState,
    makeSessionUser,
} from "../../test-utils/fixtures";
import { renderWithProviders, type ProviderOptions } from "../../test-utils/render";
import type { TemplateResponse } from "../../types/api";
import { TemplatePage } from "./TemplatePage";

const { useBoardTemplate } = vi.hoisted(() => ({ useBoardTemplate: vi.fn() }));

vi.mock("../../hooks/queries/boards", () => ({ useBoardTemplate }));

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

function renderPage(key: string, options: ProviderOptions = {}) {
    return renderWithProviders(<TemplatePage />, { route: `/templates/${key}`, path: "/templates/:key", ...options });
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    useBoardTemplate.mockReturnValue(makeQueryState<TemplateResponse>({ data: sailboat }));
});

describe("TemplatePage", () => {
    it("asks for the template the address names and shows its name and description", () => {
        // when
        renderPage("sailboat");

        // then
        expect(useBoardTemplate).toHaveBeenCalledWith("sailboat");
        expect(screen.getByRole("heading", { level: 1, name: "Sailboat" })).toBeInTheDocument();
        expect(screen.getByText("Look at what drives the team and what holds it back.")).toBeInTheDocument();
        expect(document.title).toContain("Sailboat");
    });

    it("shows every column in order with its title and description", () => {
        // when
        renderPage("sailboat");

        // then
        const columns = within(screen.getByRole("list", { name: "Sailboat columns" }));
        expect(columns.getAllByRole("heading", { level: 3 }).map(heading => heading.textContent)).toEqual([
            "Wind",
            "Anchors",
            "Rocks",
            "Island",
        ]);
        expect(columns.getByText("What held us back?")).toBeInTheDocument();
        expect(screen.getByText(/^4 columns\./)).toBeInTheDocument();
    });

    it("sends a signed out visitor who wants the template to sign up", () => {
        // when
        renderPage("sailboat");

        // then
        expect(screen.getByRole("link", { name: "Use this template" })).toHaveAttribute("href", "/register");
    });

    it("sends a signed in member who wants the template to a new board in their organisation", () => {
        // given
        const harbour = makeOrganisation({ slug: "harbour", name: "Harbour" });

        // when
        renderPage("sailboat", { user: makeSessionUser(), org: { organisations: [harbour], current: harbour } });

        // then
        expect(screen.getByRole("link", { name: "Use this template" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new",
        );
    });

    it("shows the not found page for a template the catalogue does not hold", () => {
        // given
        useBoardTemplate.mockReturnValue(
            makeQueryState<TemplateResponse>({ error: makeApiError({ status: 404, message: "template not found" }) }),
        );

        // when
        renderPage("no-such-template");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Page not found" })).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Use this template" })).not.toBeInTheDocument();
        expect(document.title).toContain("Page not found");
    });

    it("keeps the blank template off the public template pages", () => {
        // when
        renderPage("blank");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Page not found" })).toBeInTheDocument();
        expect(useBoardTemplate).toHaveBeenCalledWith("");
    });

    it("shows the server's reason when the template could not load, and tries again", async () => {
        // given
        const state = makeQueryState<TemplateResponse>({
            error: makeApiError({ status: 500, message: "internal error" }),
        });
        useBoardTemplate.mockReturnValue(state);
        const user = userEvent.setup();
        renderPage("sailboat");

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("heading", { level: 1, name: "This template could not load" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("internal error");
        expect(state.refetch).toHaveBeenCalledOnce();
    });
});
