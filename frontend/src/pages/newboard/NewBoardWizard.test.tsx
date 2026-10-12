import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardSnapshot,
    makeBoardTemplate,
    makeMutationState,
    makeOrganisation,
    makeQueryState,
    makeSavedTemplate,
    makeTeam,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, SavedTemplateListResponse, TeamResponse, TemplateListResponse } from "../../types/api";
import { NewBoardWizard } from "./NewBoardWizard";

const { useCreateBoard, useBoardTemplates, useSavedTemplates } = vi.hoisted(() => ({
    useCreateBoard: vi.fn(),
    useBoardTemplates: vi.fn(),
    useSavedTemplates: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => ({ useCreateBoard }));
vi.mock("../../hooks/queries/boards", () => ({ useBoardTemplates }));
vi.mock("../../hooks/queries/savedTemplates", () => ({ useSavedTemplates }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour" });

const NO_TEAM = "No team, everyone in the organisation";
const crew = makeTeam({ id: "team-crew", name: "Crew" });
const deck = makeTeam({ id: "team-deck", name: "Deck" });

const startStopContinue = makeBoardTemplate({
    key: "start-stop-continue",
    name: "Start, Stop, Continue",
    description: "Decide what to begin, what to drop and what to keep.",
    columns: [
        { title: "Start", description: "What should we begin doing?", colour: "green" },
        { title: "Stop", description: "What should we stop doing?", colour: "red" },
        { title: "Continue", description: "What should we keep doing?", colour: "blue" },
    ],
});
const blank = makeBoardTemplate();
const mine = makeSavedTemplate({ id: "saved-mine", name: "Our retro", description: "Saved from the sprint review." });
const platform = makeSavedTemplate({
    id: "saved-platform",
    name: "Platform format",
    description: "The team's usual columns.",
    scope: "team",
    team_id: "team-crew",
    team_name: "Crew",
});

function Landing() {
    const location = useLocation();

    return <p>landed on {location.pathname}</p>;
}

function renderWizard(
    preset: TeamResponse | null = null,
    teams: TeamResponse[] = [crew, deck],
    organisationBoards = true,
    presetTemplate: string | null = null,
) {
    return renderWithProviders(
        <Routes>
            <Route
                path="/o/:orgSlug/boards/new"
                element={
                    <NewBoardWizard
                        organisation={harbour}
                        teams={teams}
                        preset={preset}
                        noTeamPreset={false}
                        organisationBoards={organisationBoards}
                        presetTemplate={presetTemplate}
                    />
                }
            />
            <Route path="/o/:orgSlug/boards/:boardId" element={<Landing />} />
            <Route path="/o/:orgSlug" element={<Landing />} />
        </Routes>,
        { route: "/o/harbour/boards/new" },
    );
}

function progress(): string[] {
    return within(screen.getByRole("list", { name: "Progress" }))
        .getAllByRole("listitem")
        .map(step => step.textContent);
}

function currentStep(): string | null {
    return (
        within(screen.getByRole("list", { name: "Progress" }))
            .getAllByRole("listitem")
            .find(step => step.getAttribute("aria-current") === "step")?.textContent ?? null
    );
}

function created(snapshot: BoardSnapshot) {
    return (_variables: unknown, options?: { onSuccess?: (data: BoardSnapshot) => void }) => {
        options?.onSuccess?.(snapshot);
    };
}

async function reachDetails(user: ReturnType<typeof userEvent.setup>) {
    renderWizard(crew);
    await user.click(screen.getByRole("button", { name: "Next" }));
}

beforeEach(() => {
    vi.clearAllMocks();
    useCreateBoard.mockReturnValue(makeMutationState());
    useBoardTemplates.mockReturnValue(
        makeQueryState<TemplateListResponse>({ data: { templates: [startStopContinue, blank] } }),
    );
    useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ data: { templates: [] } }));
});

describe("NewBoardWizard", () => {
    it("starts by asking for the team when there are several to choose from", () => {
        // when
        renderWizard();

        // then
        expect(progress()).toEqual(["1Step 1 of 3: Team", "2Step 2 of 3: Template", "3Step 3 of 3: Details"]);
        expect(currentStep()).toBe("1Step 1 of 3: Team");
        expect(screen.getByRole("heading", { level: 2, name: "Choose a team" })).toBeInTheDocument();
        const team = screen.getByRole("combobox", { name: "Team" });
        expect(
            within(team)
                .getAllByRole("option")
                .map(option => option.textContent),
        ).toEqual([NO_TEAM, "Crew", "Deck"]);
        expect(team).toHaveDisplayValue(NO_TEAM);
        expect(screen.queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
    });

    it("skips the team step when the team is already chosen", () => {
        // when
        renderWizard(deck);

        // then
        expect(progress()).toEqual(["1Step 1 of 2: Template", "2Step 2 of 2: Details"]);
        expect(screen.getByRole("heading", { level: 2, name: "Pick a template" })).toBeInTheDocument();
        expect(screen.queryByRole("combobox", { name: "Team" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Back" })).not.toBeInTheDocument();
    });

    it("offers a way back to the boards from every step, without creating anything", async () => {
        // given
        const user = userEvent.setup();
        renderWizard();
        expect(screen.getByRole("link", { name: "Cancel" })).toHaveAttribute("href", "/o/harbour");
        await user.click(screen.getByRole("button", { name: "Next" }));
        expect(screen.getByRole("link", { name: "Cancel" })).toHaveAttribute("href", "/o/harbour");
        await user.click(screen.getByRole("button", { name: "Next" }));

        // when
        await user.click(screen.getByRole("link", { name: "Cancel" }));

        // then
        expect(screen.getByText("landed on /o/harbour")).toBeInTheDocument();
        expect(useCreateBoard().mutate).not.toHaveBeenCalled();
    });

    it("leaves focus alone on arrival, then moves it to each new step's heading", async () => {
        // given
        const user = userEvent.setup();
        renderWizard();
        expect(screen.getByRole("heading", { name: "Choose a team" })).not.toHaveFocus();

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(currentStep()).toBe("2Step 2 of 3: Template");
        expect(screen.getByRole("heading", { level: 2, name: "Pick a template" })).toHaveFocus();
    });

    it("shows every template as a choice with its columns, the first one picked", () => {
        // when
        renderWizard(crew);

        // then
        expect(screen.getByRole("group", { name: "Template" })).toBeInTheDocument();
        expect(screen.getByRole("radio", { name: "Start, Stop, Continue" })).toBeChecked();
        expect(screen.getByRole("radio", { name: "Start, Stop, Continue" })).toHaveAccessibleDescription(
            "Decide what to begin, what to drop and what to keep. Columns: Start, Stop, Continue.",
        );
        expect(screen.getByRole("radio", { name: "Blank" })).toHaveAccessibleDescription(
            "Start with one column, then rename it and add the columns you need. Columns: Notes.",
        );
    });

    it("keeps each choice when going back", async () => {
        // given
        const user = userEvent.setup();
        renderWizard();
        await user.selectOptions(screen.getByRole("combobox", { name: "Team" }), "Deck");
        await user.click(screen.getByRole("button", { name: "Next" }));
        await user.click(screen.getByRole("radio", { name: "Blank" }));
        await user.click(screen.getByRole("button", { name: "Next" }));

        // when
        await user.click(screen.getByRole("button", { name: "Back" }));

        // then
        expect(screen.getByRole("heading", { name: "Pick a template" })).toHaveFocus();
        expect(screen.getByRole("radio", { name: "Blank" })).toBeChecked();
        await user.click(screen.getByRole("button", { name: "Back" }));
        expect(screen.getByRole("combobox", { name: "Team" })).toHaveValue("team-deck");
    });

    it("sums up the team and template on the last step, starting named with cards hidden", async () => {
        // given
        const user = userEvent.setup();
        renderWizard();
        await user.selectOptions(screen.getByRole("combobox", { name: "Team" }), "Deck");
        await user.click(screen.getByRole("button", { name: "Next" }));
        await user.click(screen.getByRole("radio", { name: "Blank" }));

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(currentStep()).toBe("3Step 3 of 3: Details");
        expect(screen.getByRole("region", { name: "Name your board" })).toHaveTextContent("For Deck, using Blank.");
        expect(screen.getByRole("combobox", { name: "Card authors" })).toHaveValue("named");
        expect(screen.getByRole("switch", { name: "Show cards as they are written" })).not.toBeChecked();
    });

    it("creates the board with every choice made and opens it", async () => {
        // given
        const creation = makeMutationState({ mutate: vi.fn(created(makeBoardSnapshot({ board_id: "board-new" }))) });
        useCreateBoard.mockReturnValue(creation);
        const user = userEvent.setup();
        renderWizard();
        await user.selectOptions(screen.getByRole("combobox", { name: "Team" }), "Deck");
        await user.click(screen.getByRole("button", { name: "Next" }));
        await user.click(screen.getByRole("radio", { name: "Blank" }));
        await user.click(screen.getByRole("button", { name: "Next" }));

        // when
        await user.type(screen.getByLabelText("Board title"), "Sprint 13");
        await user.type(screen.getByLabelText("Context"), "Release week");
        await user.selectOptions(screen.getByRole("combobox", { name: "Card authors" }), "Anonymous");
        await user.click(screen.getByRole("switch", { name: "Show cards as they are written" }));
        await user.click(screen.getByRole("button", { name: "Create board" }));

        // then
        expect(creation.mutate).toHaveBeenCalledExactlyOnceWith(
            {
                orgId: "org-harbour",
                team_id: "team-deck",
                title: "Sprint 13",
                template_key: "blank",
                context: "Release week",
                anonymity: "anonymous",
                cards_shown: true,
            },
            expect.anything(),
        );
        expect(screen.getByText("landed on /o/harbour/boards/board-new")).toBeInTheDocument();
    });

    it("creates a board for the whole organisation when no team is picked", async () => {
        // given
        const creation = makeMutationState({ mutate: vi.fn(created(makeBoardSnapshot({ board_id: "board-new" }))) });
        useCreateBoard.mockReturnValue(creation);
        const user = userEvent.setup();
        renderWizard();
        await user.click(screen.getByRole("button", { name: "Next" }));
        await user.click(screen.getByRole("radio", { name: "Blank" }));
        await user.click(screen.getByRole("button", { name: "Next" }));
        expect(screen.getByRole("region", { name: "Name your board" })).toHaveTextContent(
            "For everyone in Harbour, using Blank.",
        );

        // when
        await user.type(screen.getByLabelText("Board title"), "Town hall");
        await user.click(screen.getByRole("button", { name: "Create board" }));

        // then
        expect(creation.mutate).toHaveBeenCalledExactlyOnceWith(
            {
                orgId: "org-harbour",
                title: "Town hall",
                template_key: "blank",
                context: "",
                anonymity: "named",
                cards_shown: false,
            },
            expect.anything(),
        );
        expect(screen.getByText("landed on /o/harbour/boards/board-new")).toBeInTheDocument();
    });

    it("offers only the teams, starting on the first, when the viewer may not create organisation boards", () => {
        // when
        renderWizard(null, [crew, deck], false);

        // then
        const team = screen.getByRole("combobox", { name: "Team" });
        expect(
            within(team)
                .getAllByRole("option")
                .map(option => option.textContent),
        ).toEqual(["Crew", "Deck"]);
        expect(team).toHaveValue("team-crew");
    });

    it("can be driven from the keyboard alone", async () => {
        // given
        const creation = makeMutationState({ mutate: vi.fn(created(makeBoardSnapshot({ board_id: "board-new" }))) });
        useCreateBoard.mockReturnValue(creation);
        const user = userEvent.setup();
        renderWizard(crew);

        // when
        await user.tab();
        await user.keyboard("{ArrowDown}");
        await user.tab();
        await user.keyboard("{Enter}");
        await user.tab();
        await user.keyboard("Sprint 13{Enter}");

        // then
        expect(creation.mutate).toHaveBeenCalledExactlyOnceWith(
            expect.objectContaining({
                orgId: "org-harbour",
                team_id: "team-crew",
                title: "Sprint 13",
                template_key: "blank",
            }),
            expect.anything(),
        );
    });

    it("says what is missing, ties it to the field and moves focus to it, without creating anything", async () => {
        // given
        const creation = makeMutationState();
        useCreateBoard.mockReturnValue(creation);
        const user = userEvent.setup();
        await reachDetails(user);

        // when
        await user.type(screen.getByLabelText("Board title"), "   ");
        await user.click(screen.getByRole("button", { name: "Create board" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a board title.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByLabelText("Board title")).toBeInvalid();
        expect(creation.mutate).not.toHaveBeenCalled();
        expect(creation.reset).toHaveBeenCalled();
    });

    it("holds the button while the board is created", async () => {
        // given
        useCreateBoard.mockReturnValue(makeMutationState({ isPending: true }));
        const user = userEvent.setup();

        // when
        await reachDetails(user);

        // then
        expect(screen.getByRole("button", { name: "Create board" })).toBeDisabled();
    });

    it.each([
        { field: "Board title", status: 400, message: "a board title is required", code: "board_title_required" },
        {
            field: "Board title",
            status: 400,
            message: "a board title can be at most 200 characters",
            code: "board_title_too_long",
        },
        {
            field: "Context",
            status: 400,
            message: "the context can be at most 500 characters",
            code: "board_context_too_long",
        },
        { field: "Card authors", status: 400, message: "unknown anonymity", code: "invalid_anonymity" },
    ])("ties the server's refusal to the field it is about: $code", async ({ field, ...refusal }) => {
        // given
        useCreateBoard.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));
        const user = userEvent.setup();

        // when
        await reachDetails(user);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText(field)).toHaveAccessibleDescription(expect.stringContaining(refusal.message));
    });

    it("ties a template refusal to the template choice", async () => {
        // given
        const refusal = { status: 400, message: "unknown template", code: "unknown_template" };
        useCreateBoard.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));
        const user = userEvent.setup();
        await reachDetails(user);
        expect(screen.getByRole("alert")).toHaveTextContent("unknown template");

        // when
        await user.click(screen.getByRole("button", { name: "Back" }));

        // then
        expect(screen.getByRole("group", { name: "Template" })).toHaveAccessibleDescription("unknown template");
    });

    it.each([
        { status: 409, message: "the team is archived, so it can no longer be changed", code: "team_archived" },
        { status: 404, message: "team not found" },
    ])("shows any other failure as it comes, without blaming a field: $message", async refusal => {
        // given
        useCreateBoard.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));
        const user = userEvent.setup();

        // when
        await reachDetails(user);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Board title")).not.toHaveAttribute("aria-invalid");
    });

    it("says it is loading while the templates are on their way, and holds Next", () => {
        // given
        useBoardTemplates.mockReturnValue(makeQueryState<TemplateListResponse>({ loading: true }));

        // when
        renderWizard(crew);

        // then
        expect(screen.getByText("Loading templates...")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    });

    it("says when no templates are available, and holds Next", () => {
        // given
        useBoardTemplates.mockReturnValue(makeQueryState<TemplateListResponse>({ data: { templates: [] } }));

        // when
        renderWizard(crew);

        // then
        expect(screen.getByText("No templates are available.")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    });

    it("shows the server's reason when the templates could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<TemplateListResponse>({
            error: makeApiError({ status: 503, message: "session store unavailable" }),
        });
        useBoardTemplates.mockReturnValue(failed);
        const user = userEvent.setup();
        renderWizard(crew);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("session store unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("shows your templates above the built-in ones, naming the team a template is shared with", () => {
        // given
        useSavedTemplates.mockReturnValue(
            makeQueryState<SavedTemplateListResponse>({ data: { templates: [mine, platform] } }),
        );

        // when
        renderWizard(crew);

        // then the first built-in template stays picked
        const yours = screen.getByRole("group", { name: "Your templates" });
        expect(within(yours).getByRole("radio", { name: "Our retro" })).toHaveAccessibleDescription(
            "Only you Saved from the sprint review. Columns: Went well, To improve.",
        );
        expect(within(yours).getByRole("radio", { name: "Platform format" })).toHaveAccessibleDescription(
            "Shared with Crew The team's usual columns. Columns: Went well, To improve.",
        );
        const builtIn = screen.getByRole("group", { name: "Built-in templates" });
        expect(within(builtIn).getByRole("radio", { name: "Start, Stop, Continue" })).toBeChecked();
    });

    it("creates the board from a saved template", async () => {
        // given
        useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ data: { templates: [mine] } }));
        const creation = makeMutationState({ mutate: vi.fn(created(makeBoardSnapshot({ board_id: "board-new" }))) });
        useCreateBoard.mockReturnValue(creation);
        const user = userEvent.setup();
        renderWizard(crew);
        await user.click(screen.getByRole("radio", { name: "Our retro" }));
        await user.click(screen.getByRole("button", { name: "Next" }));
        expect(screen.getByRole("region", { name: "Name your board" })).toHaveTextContent("For Crew, using Our retro.");

        // when
        await user.type(screen.getByLabelText("Board title"), "Sprint 14");
        await user.click(screen.getByRole("button", { name: "Create board" }));

        // then
        expect(creation.mutate).toHaveBeenCalledExactlyOnceWith(
            {
                orgId: "org-harbour",
                team_id: "team-crew",
                title: "Sprint 14",
                template_key: "",
                saved_template_id: "saved-mine",
                context: "",
                anonymity: "named",
                cards_shown: false,
            },
            expect.anything(),
        );
    });

    it("starts on the saved template the page asked for", () => {
        // given
        useSavedTemplates.mockReturnValue(
            makeQueryState<SavedTemplateListResponse>({ data: { templates: [mine, platform] } }),
        );

        // when
        renderWizard(crew, [crew, deck], true, "saved-platform");

        // then
        expect(screen.getByRole("radio", { name: "Platform format" })).toBeChecked();
        expect(screen.getByRole("radio", { name: "Start, Stop, Continue" })).not.toBeChecked();
    });

    it("waits for your templates before offering the choice, and holds Next", () => {
        // given
        useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ loading: true }));

        // when
        renderWizard(crew);

        // then
        expect(screen.getByText("Loading templates...")).toBeInTheDocument();
        expect(screen.queryByRole("radio", { name: "Start, Stop, Continue" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
    });

    it("still offers the built-in templates when your templates could not load", async () => {
        // given
        const failed = makeQueryState<SavedTemplateListResponse>({
            error: makeApiError({ status: 503, message: "database unavailable" }),
        });
        useSavedTemplates.mockReturnValue(failed);
        const user = userEvent.setup();
        renderWizard(crew);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("database unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
        expect(screen.getByRole("radio", { name: "Start, Stop, Continue" })).toBeChecked();
    });
});
