import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeSurvey,
    makeSurveyQuestion,
    type MutationStateFixture,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { SurveyView } from "../../../types/api";
import { SurveyCard } from "./SurveyCard";

const hooks = vi.hoisted(() => ({
    useCloseSurvey: vi.fn(),
    useDeleteSurvey: vi.fn(),
    useRespondToSurvey: vi.fn(),
}));

vi.mock("../../../hooks/mutations/surveys", () => ({
    useCloseSurvey: hooks.useCloseSurvey,
    useDeleteSurvey: hooks.useDeleteSurvey,
    useRespondToSurvey: hooks.useRespondToSurvey,
}));

const pace = makeSurveyQuestion({ id: "q-pace", prompt: "How sustainable was the pace?" });
const mood = makeSurveyQuestion({
    id: "q-mood",
    kind: "single_choice",
    prompt: "How do you feel?",
    options: ["Good", "Okay", "Bad"],
    scale_min: null,
    scale_max: null,
});
const focus = makeSurveyQuestion({
    id: "q-focus",
    kind: "multiple_choice",
    prompt: "What should we focus on?",
    options: ["Quality", "Speed", "Process"],
    scale_min: null,
    scale_max: null,
});

const open = makeSurvey({ id: "s1", title: "Sprint health", questions: [pace, mood, focus], respondents: 3 });

let respond: MutationStateFixture;
let closer: MutationStateFixture;
let remover: MutationStateFixture;

function renderCard(survey: SurveyView, { canRespond = true, canManage = false } = {}) {
    return renderWithProviders(
        <SurveyCard boardId="b1" survey={survey} canRespond={canRespond} canManage={canManage} />,
    );
}

function card(title = "Sprint health") {
    return within(screen.getByRole("article", { name: title }));
}

beforeEach(() => {
    vi.clearAllMocks();
    respond = makeMutationState();
    closer = makeMutationState();
    remover = makeMutationState();
    hooks.useRespondToSurvey.mockReturnValue(respond);
    hooks.useCloseSurvey.mockReturnValue(closer);
    hooks.useDeleteSurvey.mockReturnValue(remover);
});

describe("SurveyCard", () => {
    it("shows the title, that it is open and how many people answered", () => {
        // when
        renderCard(open);

        // then
        expect(card().getByText("Open")).toBeInTheDocument();
        expect(card().getByText("3 people answered")).toBeInTheDocument();
    });

    it("asks each question in its own way and sends the answers once every one is given", async () => {
        // given
        const user = userEvent.setup();
        renderCard(open);
        const submit = card().getByRole("button", { name: "Submit answers" });
        expect(submit).toBeDisabled();

        // when
        const scale = within(card().getByRole("group", { name: "How sustainable was the pace?" }));
        await user.click(scale.getByRole("button", { name: "4" }));
        await user.click(card().getByRole("radio", { name: "Okay" }));
        await user.click(card().getByRole("checkbox", { name: "Process" }));
        await user.click(card().getByRole("checkbox", { name: "Quality" }));
        await user.click(submit);

        // then
        expect(card().getByRole("button", { name: "4" })).toHaveAttribute("aria-pressed", "true");
        expect(respond.mutate).toHaveBeenCalledWith({
            surveyId: "s1",
            answers: [
                { question_id: "q-pace", answer: [4] },
                { question_id: "q-mood", answer: [1] },
                { question_id: "q-focus", answer: [0, 2] },
            ],
        });
    });

    it("fills in the viewer's own answers and says a new submission replaces them", () => {
        // when
        renderCard({ ...open, my_answers: { "q-pace": [2], "q-mood": [0], "q-focus": [1] } });

        // then
        expect(card().getByRole("button", { name: "2" })).toHaveAttribute("aria-pressed", "true");
        expect(card().getByRole("button", { name: "4" })).toHaveAttribute("aria-pressed", "false");
        expect(card().getByRole("radio", { name: "Good" })).toBeChecked();
        expect(card().getByRole("checkbox", { name: "Speed" })).toBeChecked();
        expect(card().getByText("You have answered. Submitting again replaces your answers.")).toBeInTheDocument();
        expect(card().getByRole("button", { name: "Submit answers" })).toBeEnabled();
    });

    it("lists the questions without results to someone who cannot answer", () => {
        // when
        renderCard(open, { canRespond: false });

        // then
        expect(card().queryByRole("button", { name: "Submit answers" })).not.toBeInTheDocument();
        expect(card().getByText("How do you feel?")).toBeInTheDocument();
        expect(card().getByText("Results appear once the survey is closed.")).toBeInTheDocument();
    });

    it("shows the results of a closed survey", () => {
        // given
        const closed = makeSurvey({
            title: "Team mood",
            status: "closed",
            respondents: 2,
            questions: [{ ...mood, results: { counts: [2, 0, 0], responses: 2, average: null } }],
        });

        // when
        renderCard(closed);

        // then
        expect(card("Team mood").getByText("Closed")).toBeInTheDocument();
        expect(card("Team mood").getByRole("listitem", { name: "Good: 2" })).toBeInTheDocument();
        expect(card("Team mood").queryByRole("button", { name: "Submit answers" })).not.toBeInTheDocument();
    });

    it("lets a facilitator close the survey", async () => {
        // given
        const user = userEvent.setup();
        renderCard(open, { canManage: true });

        // when
        await user.click(card().getByRole("button", { name: "Close survey" }));

        // then
        expect(closer.mutate).toHaveBeenCalledWith({ surveyId: "s1" });
    });

    it("asks a facilitator to confirm before deleting the survey", async () => {
        // given
        const user = userEvent.setup();
        renderCard(open, { canManage: true });

        // when
        await user.click(card().getByRole("button", { name: "Delete" }));
        const dialog = screen.getByRole("dialog", { name: "Delete this survey?" });
        await user.click(within(dialog).getByRole("button", { name: "Delete survey" }));

        // then
        expect(dialog).toHaveTextContent(
            '"Sprint health" and every answer to it are removed for everyone on the board.',
        );
        expect(remover.mutate).toHaveBeenCalledWith({ surveyId: "s1" }, expect.anything());
    });

    it("offers no close to a survey already closed", () => {
        // when
        renderCard({ ...open, status: "closed" }, { canManage: true });

        // then
        expect(card().queryByRole("button", { name: "Close survey" })).not.toBeInTheDocument();
        expect(card().getByRole("button", { name: "Delete" })).toBeInTheDocument();
    });

    it("explains why an answer was refused", () => {
        // given
        hooks.useRespondToSurvey.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "the survey is closed", code: "survey_closed" }),
            }),
        );

        // when
        renderCard(open);

        // then
        expect(card().getByRole("alert")).toHaveTextContent("This survey has closed, so it takes no more answers.");
    });
});
