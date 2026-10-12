import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeQueryState,
    makeSavedTemplate,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { SavedTemplateListResponse } from "../../../types/api";
import { SavedTemplatesPage } from "./SavedTemplatesPage";

const { useSavedTemplates, useDeleteSavedTemplate } = vi.hoisted(() => ({
    useSavedTemplates: vi.fn(),
    useDeleteSavedTemplate: vi.fn(),
}));

vi.mock("../../../hooks/queries/savedTemplates", () => ({ useSavedTemplates }));
vi.mock("../../../hooks/mutations/savedTemplates", () => ({ useDeleteSavedTemplate }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour" });

const mine = makeSavedTemplate({ id: "saved-mine", name: "Our retro", description: "Saved from the sprint review." });
const crew = makeSavedTemplate({
    id: "saved-crew",
    name: "Crew format",
    description: "",
    scope: "team",
    team_id: "team-crew",
    team_name: "Crew",
    can_delete: false,
});

function showTemplates(...templates: SavedTemplateListResponse["templates"]) {
    useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ data: { templates } }));
}

function renderPage(deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, deleted_at: deletedAt });

    return renderWithProviders(<SavedTemplatesPage />, {
        user: makeSessionUser(),
        org: { organisations: [current], current, role: "member" },
        route: "/o/harbour/templates",
        path: "/o/:orgSlug/templates",
    });
}

function card(name: string): HTMLElement {
    return screen.getByRole("listitem", { name });
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showTemplates(mine, crew);
    useDeleteSavedTemplate.mockReturnValue(makeMutationState());
});

describe("SavedTemplatesPage", () => {
    it("heads the page and asks for the organisation's saved templates", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Templates" })).toBeInTheDocument();
        expect(useSavedTemplates).toHaveBeenCalledWith("org-harbour");
        expect(useDeleteSavedTemplate).toHaveBeenCalledWith("org-harbour");
        expect(document.title).toContain("Templates");
    });

    it("shows each template as a card with who it is for and its columns", () => {
        // when
        renderPage();

        // then
        const list = screen.getByRole("list", { name: "Saved templates" });
        expect(within(list).getAllByRole("listitem")).toHaveLength(2);
        expect(card("Our retro")).toHaveTextContent("Only you");
        expect(card("Our retro")).toHaveTextContent("Saved from the sprint review.");
        expect(card("Our retro")).toHaveTextContent("Columns: Went well, To improve.");
        expect(card("Crew format")).toHaveTextContent("Shared with Crew");
    });

    it("starts a new board from a template", () => {
        // when
        renderPage();

        // then
        expect(within(card("Our retro")).getByRole("link", { name: "Use template" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?saved=saved-mine",
        );
    });

    it("offers Delete only on a template the viewer may delete", () => {
        // when
        renderPage();

        // then
        expect(within(card("Our retro")).getByRole("button", { name: "Delete" })).toBeInTheDocument();
        expect(within(card("Crew format")).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
    });

    it("asks before deleting, then deletes the template", async () => {
        // given
        const removal = makeMutationState();
        useDeleteSavedTemplate.mockReturnValue(removal);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(within(card("Our retro")).getByRole("button", { name: "Delete" }));
        const dialog = screen.getByRole("dialog", { name: "Delete template?" });
        await user.click(within(dialog).getByRole("button", { name: "Delete template" }));

        // then
        expect(removal.mutate).toHaveBeenCalledExactlyOnceWith("saved-mine", expect.anything());
    });

    it("deletes nothing when the question is cancelled", async () => {
        // given
        const removal = makeMutationState();
        useDeleteSavedTemplate.mockReturnValue(removal);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(within(card("Our retro")).getByRole("button", { name: "Delete" }));
        await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Cancel" }));

        // then
        expect(removal.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("shows the server's reason when a delete is refused", () => {
        // given
        useDeleteSavedTemplate.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 403,
                    message: "you cannot delete this template",
                    code: "delete_not_allowed",
                }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("you cannot delete this template");
    });

    it("explains how to save a template when there are none", () => {
        // given
        showTemplates();

        // when
        renderPage();

        // then
        expect(screen.getByText(/No saved templates yet/)).toBeInTheDocument();
        expect(screen.queryByRole("list", { name: "Saved templates" })).not.toBeInTheDocument();
    });

    it("says it is loading while the templates are on their way", () => {
        // given
        useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading templates...")).toBeInTheDocument();
    });

    it("shows the server's reason when the templates could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<SavedTemplateListResponse>({
            error: makeApiError({ status: 503, message: "database unavailable" }),
        });
        useSavedTemplates.mockReturnValue(failed);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("database unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("says the organisation is deleted instead of asking for its templates", () => {
        // when
        renderPage("2026-09-30T09:00:00Z");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useSavedTemplates).not.toHaveBeenCalled();
    });
});
