import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeMutationState,
    makeParticipantViewer,
    makeQueryState,
    makeSurvey,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BoardSnapshot } from "../../../types/api";
import { SurveysPanel } from "./SurveysPanel";

const hooks = vi.hoisted(() => ({
    useCreateSurvey: vi.fn(),
    useCloseSurvey: vi.fn(),
    useDeleteSurvey: vi.fn(),
    useRespondToSurvey: vi.fn(),
    useSurveyTemplates: vi.fn(),
}));

vi.mock("../../../hooks/mutations/surveys", () => ({
    useCreateSurvey: hooks.useCreateSurvey,
    useCloseSurvey: hooks.useCloseSurvey,
    useDeleteSurvey: hooks.useDeleteSurvey,
    useRespondToSurvey: hooks.useRespondToSurvey,
}));
vi.mock("../../../hooks/queries/surveys", () => ({ useSurveyTemplates: hooks.useSurveyTemplates }));

const health = makeSurvey({ id: "s1", title: "Sprint health" });
const mood = makeSurvey({ id: "s2", title: "Team mood", status: "closed" });

function renderPanel(snapshot: BoardSnapshot, writable = true) {
    return renderWithProviders(<SurveysPanel snapshot={snapshot} writable={writable} />);
}

function surveysRegion() {
    return screen.getByRole("region", { name: "Surveys" });
}

beforeEach(() => {
    vi.clearAllMocks();
    hooks.useCreateSurvey.mockReturnValue(makeMutationState());
    hooks.useCloseSurvey.mockReturnValue(makeMutationState());
    hooks.useDeleteSurvey.mockReturnValue(makeMutationState());
    hooks.useRespondToSurvey.mockReturnValue(makeMutationState());
    hooks.useSurveyTemplates.mockReturnValue(makeQueryState({ data: { templates: [] } }));
});

describe("SurveysPanel", () => {
    it("draws nothing for a participant while the board has no surveys", () => {
        // when
        renderPanel(makeBoardSnapshot({ viewer: makeParticipantViewer(), surveys: [] }));

        // then
        expect(screen.queryByRole("region", { name: "Surveys" })).not.toBeInTheDocument();
    });

    it("invites a facilitator to start the first survey", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(makeBoardSnapshot({ surveys: [] }));

        // when
        await user.click(within(surveysRegion()).getByRole("button", { name: "New survey" }));

        // then
        expect(surveysRegion()).toHaveTextContent("No surveys yet.");
        expect(screen.getByRole("dialog", { name: "New survey" })).toBeInTheDocument();
    });

    it("lays every survey out as a tile in the grid", () => {
        // when
        renderPanel(makeBoardSnapshot({ surveys: [health, mood] }));

        // then
        const tiles = within(surveysRegion()).getAllByRole("listitem");
        expect(tiles).toHaveLength(2);
        expect(within(tiles[0]).getByRole("article", { name: "Sprint health" })).toBeInTheDocument();
        expect(within(tiles[1]).getByRole("article", { name: "Team mood" })).toBeInTheDocument();
    });

    it("lets a participant answer but never manage", () => {
        // when
        renderPanel(makeBoardSnapshot({ viewer: makeParticipantViewer(), surveys: [health] }));

        // then
        const region = surveysRegion();
        expect(within(region).queryByRole("button", { name: "New survey" })).not.toBeInTheDocument();
        expect(within(region).queryByRole("button", { name: "Close survey" })).not.toBeInTheDocument();
        expect(within(region).getByRole("button", { name: "Submit answers" })).toBeInTheDocument();
    });

    it("offers nothing to change while the board cannot be written to", () => {
        // when
        renderPanel(makeBoardSnapshot({ surveys: [health] }), false);

        // then
        const region = surveysRegion();
        expect(within(region).queryByRole("button", { name: "New survey" })).not.toBeInTheDocument();
        expect(within(region).queryByRole("button", { name: "Submit answers" })).not.toBeInTheDocument();
        expect(region).toHaveTextContent("Results appear once the survey is closed.");
    });

    it("takes no answers while the board is locked", () => {
        // when
        renderPanel(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                settings: makeBoardSettings({ board_locked: true }),
                surveys: [health],
            }),
        );

        // then
        expect(within(surveysRegion()).queryByRole("button", { name: "Submit answers" })).not.toBeInTheDocument();
    });

    it("shows an ended board's surveys to read, with nothing to change", () => {
        // when
        renderPanel(makeBoardSnapshot({ board: makeBoardView({ closed: true, stage: "end" }), surveys: [mood] }));

        // then
        const region = surveysRegion();
        expect(within(region).getByRole("article", { name: "Team mood" })).toBeInTheDocument();
        expect(within(region).queryByRole("button")).not.toBeInTheDocument();
    });
});
