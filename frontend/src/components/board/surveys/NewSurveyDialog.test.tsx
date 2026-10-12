import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeQueryState,
    type MutationStateFixture,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { SurveyTemplate } from "../../../types/api";
import { NewSurveyDialog } from "./NewSurveyDialog";

const hooks = vi.hoisted(() => ({
    useCreateSurvey: vi.fn(),
    useSurveyTemplates: vi.fn(),
}));

vi.mock("../../../hooks/mutations/surveys", () => ({ useCreateSurvey: hooks.useCreateSurvey }));
vi.mock("../../../hooks/queries/surveys", () => ({ useSurveyTemplates: hooks.useSurveyTemplates }));

const healthCheck: SurveyTemplate = {
    key: "health_check",
    title: "Team health check",
    description: "Eight quick ratings of how the team is doing.",
    category: "health_check",
    questions: [{ kind: "scale", prompt: "Fun?", options: [], scale_min: 1, scale_max: 5 }],
};

const teamMood: SurveyTemplate = {
    key: "team_mood",
    title: "Team mood",
    description: "A single pick of how everyone is feeling.",
    category: "survey",
    questions: [
        { kind: "single_choice", prompt: "Mood?", options: ["Great", "Bad"], scale_min: null, scale_max: null },
    ],
};

let create: MutationStateFixture;
let onClose: Mock<() => void>;

function renderDialog() {
    return renderWithProviders(<NewSurveyDialog boardId="b1" onClose={onClose} />);
}

function dialog() {
    return within(screen.getByRole("dialog", { name: "New survey" }));
}

function question(position: number) {
    return within(dialog().getByRole("group", { name: `Question ${position}` }));
}

beforeEach(() => {
    vi.clearAllMocks();
    create = makeMutationState();
    onClose = vi.fn<() => void>();
    hooks.useCreateSurvey.mockReturnValue(create);
    hooks.useSurveyTemplates.mockReturnValue(makeQueryState({ data: { templates: [teamMood, healthCheck] } }));
});

describe("NewSurveyDialog", () => {
    it("groups the templates by category, health checks first, beside a custom survey", () => {
        // when
        renderDialog();

        // then
        const health = within(dialog().getByRole("region", { name: "Health check" }));
        const surveys = within(dialog().getByRole("region", { name: "Survey" }));
        expect(health.getByRole("button", { name: /Team health check/ })).toBeInTheDocument();
        expect(surveys.getByRole("button", { name: /Team mood/ })).toBeInTheDocument();
        expect(dialog().getByRole("button", { name: /Custom survey/ })).toBeInTheDocument();
    });

    it("creates a survey straight from a template, under the template's title", async () => {
        // given
        const user = userEvent.setup();
        create.mutate.mockImplementation((_variables, options: { onSuccess: () => void }) => options.onSuccess());
        renderDialog();

        // when
        await user.click(dialog().getByRole("button", { name: /Team mood/ }));

        // then
        expect(create.mutate).toHaveBeenCalledWith(
            { title: "Team mood", template_key: "team_mood" },
            expect.objectContaining({ onSuccess: onClose }),
        );
        expect(onClose).toHaveBeenCalled();
    });

    it("builds a custom survey from its own questions", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        await user.click(dialog().getByRole("button", { name: /Custom survey/ }));

        // when
        await user.type(dialog().getByLabelText("Title"), " Retro check ");
        await user.type(question(1).getByLabelText("Prompt"), "How do you feel?");
        await user.type(question(1).getByLabelText("Option 1"), "Good");
        await user.type(question(1).getByLabelText("Option 2"), "Bad");
        await user.click(dialog().getByRole("button", { name: "Add question" }));
        await user.selectOptions(question(2).getByLabelText("Kind"), "scale");
        await user.type(question(2).getByLabelText("Prompt"), "Pace?");
        await user.click(dialog().getByRole("button", { name: "Create survey" }));

        // then
        expect(create.mutate).toHaveBeenCalledWith(
            {
                title: "Retro check",
                questions: [
                    {
                        kind: "single_choice",
                        prompt: "How do you feel?",
                        options: ["Good", "Bad"],
                        scale_min: null,
                        scale_max: null,
                    },
                    { kind: "scale", prompt: "Pace?", options: [], scale_min: 1, scale_max: 5 },
                ],
            },
            expect.objectContaining({ onSuccess: onClose }),
        );
    });

    it("adds and removes options and questions within the limits", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        await user.click(dialog().getByRole("button", { name: /Custom survey/ }));

        // when
        await user.click(question(1).getByRole("button", { name: "Add option" }));
        await user.click(question(1).getByRole("button", { name: "Remove option 1" }));
        await user.click(dialog().getByRole("button", { name: "Add question" }));
        await user.click(question(2).getByRole("button", { name: "Remove question" }));

        // then
        expect(question(1).getAllByRole("textbox", { name: /Option/ })).toHaveLength(2);
        expect(question(1).getByRole("button", { name: "Remove option 1" })).toBeDisabled();
        expect(dialog().queryByRole("group", { name: "Question 2" })).not.toBeInTheDocument();
        expect(question(1).queryByRole("button", { name: "Remove question" })).not.toBeInTheDocument();
    });

    it("shows a scale's ends when a question becomes a scale", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        await user.click(dialog().getByRole("button", { name: /Custom survey/ }));

        // when
        await user.selectOptions(question(1).getByLabelText("Kind"), "scale");

        // then
        expect(question(1).getByLabelText("Lowest score")).toHaveValue(1);
        expect(question(1).getByLabelText("Highest score")).toHaveValue(5);
        expect(question(1).queryByLabelText("Option 1")).not.toBeInTheDocument();
    });

    it("says what is missing and sends nothing", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        await user.click(dialog().getByRole("button", { name: /Custom survey/ }));
        await user.type(dialog().getByLabelText("Title"), "Retro check");

        // when
        await user.click(dialog().getByRole("button", { name: "Create survey" }));

        // then
        expect(dialog().getByRole("alert")).toHaveTextContent("Question 1 needs a prompt.");
        expect(create.mutate).not.toHaveBeenCalled();
    });

    it("goes back to the templates", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        await user.click(dialog().getByRole("button", { name: /Custom survey/ }));

        // when
        await user.click(dialog().getByRole("button", { name: "Back" }));

        // then
        expect(dialog().getByRole("button", { name: /Team mood/ })).toBeInTheDocument();
    });

    it("explains why the server refused the survey", () => {
        // given
        hooks.useCreateSurvey.mockReturnValue(
            makeMutationState({ error: makeApiError({ status: 409, message: "too many", code: "too_many_surveys" }) }),
        );

        // when
        renderDialog();

        // then
        expect(dialog().getByRole("alert")).toHaveTextContent("A board can hold at most 20 surveys.");
    });

    it("offers a retry when the templates could not be read", async () => {
        // given
        const user = userEvent.setup();
        const refetch = vi.fn().mockResolvedValue(undefined);
        hooks.useSurveyTemplates.mockReturnValue(
            makeQueryState({ error: new Error("the catalogue is down"), refetch }),
        );
        renderDialog();

        // when
        await user.click(dialog().getByRole("button", { name: "Try again" }));

        // then
        expect(dialog().getByText("the catalogue is down")).toBeInTheDocument();
        expect(refetch).toHaveBeenCalled();
        expect(dialog().getByRole("button", { name: /Custom survey/ })).toBeInTheDocument();
    });
});
