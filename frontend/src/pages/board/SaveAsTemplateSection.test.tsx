import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardView,
    makeMutationState,
    makeOrganisation,
    makeSavedTemplate,
    makeUpgradeRequiredError,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { SavedTemplateResponse } from "../../types/api";
import { SaveAsTemplateSection } from "./SaveAsTemplateSection";

const { useSaveBoardAsTemplate } = vi.hoisted(() => ({
    useSaveBoardAsTemplate: vi.fn(),
}));

vi.mock("../../hooks/mutations/savedTemplates", () => ({ useSaveBoardAsTemplate }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour" });
const teamBoard = makeBoardView({ id: "board-1", team_id: "team-crew" });
const organisationBoard = makeBoardView({ id: "board-2", team_id: undefined });

function saving(template: SavedTemplateResponse) {
    return (_variables: unknown, options?: { onSuccess?: (data: SavedTemplateResponse) => void }) => {
        options?.onSuccess?.(template);
    };
}

beforeEach(() => {
    vi.clearAllMocks();
    useSaveBoardAsTemplate.mockReturnValue(makeMutationState());
});

describe("SaveAsTemplateSection", () => {
    it("saves the board's columns as a template for the team, then links to the templates", async () => {
        // given
        const save = makeMutationState({ mutate: vi.fn(saving(makeSavedTemplate({ name: "Crew retro" }))) });
        useSaveBoardAsTemplate.mockReturnValue(save);
        const user = userEvent.setup();
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={teamBoard} />);

        // when
        await user.type(screen.getByLabelText("Template name"), "Crew retro");
        await user.type(screen.getByLabelText("Template description"), "Our usual columns");
        await user.selectOptions(screen.getByRole("combobox", { name: "Who can use it" }), "team");
        await user.click(screen.getByRole("button", { name: "Save as template" }));

        // then
        expect(save.mutate).toHaveBeenCalledExactlyOnceWith(
            {
                orgId: "org-harbour",
                boardId: "board-1",
                name: "Crew retro",
                description: "Our usual columns",
                scope: "team",
            },
            expect.anything(),
        );
        const status = screen.getByRole("status");
        expect(status).toHaveTextContent("Saved Crew retro.");
        expect(within(status).getByRole("link", { name: "See your templates" })).toHaveAttribute(
            "href",
            "/o/harbour/templates",
        );
        expect(screen.getByLabelText("Template name")).toHaveValue("");
    });

    it("offers the team only on a team board, and saves for the saver otherwise", async () => {
        // given
        const save = makeMutationState();
        useSaveBoardAsTemplate.mockReturnValue(save);
        const user = userEvent.setup();
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={organisationBoard} />);

        // when
        await user.type(screen.getByLabelText("Template name"), "All hands");
        await user.click(screen.getByRole("button", { name: "Save as template" }));

        // then
        expect(screen.queryByRole("combobox", { name: "Who can use it" })).not.toBeInTheDocument();
        expect(save.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", boardId: "board-2", name: "All hands", description: "", scope: "personal" },
            expect.anything(),
        );
    });

    it("lists who can use a team board's template", () => {
        // when
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={teamBoard} />);

        // then
        const scope = screen.getByRole("combobox", { name: "Who can use it" });
        expect(
            within(scope)
                .getAllByRole("option")
                .map(option => option.textContent),
        ).toEqual(["Only me", "Everyone on the board's team"]);
        expect(scope).toHaveValue("personal");
    });

    it("says a name is missing and ties it to the field, without saving", async () => {
        // given
        const save = makeMutationState();
        useSaveBoardAsTemplate.mockReturnValue(save);
        const user = userEvent.setup();
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={teamBoard} />);

        // when
        await user.click(screen.getByRole("button", { name: "Save as template" }));

        // then
        expect(save.mutate).not.toHaveBeenCalled();
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a template name.");
        expect(screen.getByLabelText("Template name")).toHaveAccessibleDescription(
            expect.stringContaining("Enter a template name."),
        );
    });

    it("ties the server's refusal to the field it is about", () => {
        // given
        const refusal = {
            status: 400,
            message: "a template name can be at most 80 characters",
            code: "template_name_too_long",
        };
        useSaveBoardAsTemplate.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={teamBoard} />);

        // then
        expect(screen.getByLabelText("Template name")).toHaveAttribute("aria-invalid", "true");
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
    });

    it("offers an upgrade when the plan's saved templates are used up", () => {
        // given
        const upgrade = makeUpgradeRequiredError({ limit: "saved_templates", allowed: 3, current: 3 });
        useSaveBoardAsTemplate.mockReturnValue(makeMutationState({ isError: true, error: upgrade }));

        // when
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={teamBoard} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("allows 3 saved templates");
    });

    it("holds the button while the template is saved", () => {
        // given
        useSaveBoardAsTemplate.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<SaveAsTemplateSection organisation={harbour} board={teamBoard} />);

        // then
        expect(screen.getByRole("button", { name: "Save as template" })).toBeDisabled();
    });
});
