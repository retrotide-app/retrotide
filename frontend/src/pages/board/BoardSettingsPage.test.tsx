import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SYNC_LIVE } from "../../domain/board/syncStatus";
import {
    BOARD_ID,
    makeBoardParticipant,
    makeBoardSnapshot,
    makeBoardView,
    makeBoardViewer,
    makeMutationState,
    makeOrganisation,
    makeParticipantViewer,
    makeQueryState,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, TeamListResponse } from "../../types/api";
import { BoardSettingsPage } from "./BoardSettingsPage";

const { useBoardView } = vi.hoisted(() => ({ useBoardView: vi.fn() }));

const mutations = vi.hoisted(() => ({
    useJoinBoard: vi.fn(),
    useDescribeBoard: vi.fn(),
    useCreateColumn: vi.fn(),
    useUpdateColumn: vi.fn(),
    useMoveColumn: vi.fn(),
    useClearColumn: vi.fn(),
    useDeleteColumn: vi.fn(),
    useUpdateBoardSettings: vi.fn(),
    useResetVotes: vi.fn(),
    useRevealCards: vi.fn(),
    useHideCards: vi.fn(),
    useSetParticipantRole: vi.fn(),
    useRemoveParticipant: vi.fn(),
}));

const management = vi.hoisted(() => ({
    useArchiveBoard: vi.fn(),
    useUnarchiveBoard: vi.fn(),
    useDeleteBoard: vi.fn(),
    useMoveBoard: vi.fn(),
    useImportCsv: vi.fn(),
}));

const { useTeams, BoardBackgroundSection } = vi.hoisted(() => ({
    useTeams: vi.fn(),
    BoardBackgroundSection: vi.fn((_props: { snapshot: BoardSnapshot }) => null),
}));

vi.mock("../../hooks/board/useBoardView", () => ({ useBoardView }));
vi.mock("../../hooks/mutations/boards", () => mutations);
vi.mock("../../hooks/mutations/boardManagement", () => management);
vi.mock("../../hooks/queries/teams", () => ({ useTeams }));
vi.mock("./BoardBackgroundSection", () => ({ BoardBackgroundSection }));

const harbour = makeOrganisation({ slug: "harbour" });

function showBoard(snapshot: BoardSnapshot) {
    useBoardView.mockReturnValue({ ...makeQueryState<BoardSnapshot>({ data: snapshot }), sync: SYNC_LIVE });
}

function renderPage() {
    return renderWithProviders(<BoardSettingsPage />, {
        org: { organisations: [harbour], current: harbour },
        route: `/o/harbour/boards/${BOARD_ID}/settings`,
        path: "/o/:orgSlug/boards/:boardId/settings",
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    for (const hook of Object.values(management)) {
        hook.mockReturnValue(makeMutationState());
    }
    useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ data: { teams: [] } }));
    mutations.useJoinBoard.mockReturnValue(makeMutationState({ isSuccess: true }));
    showBoard(makeBoardSnapshot());
});

describe("BoardSettingsPage", () => {
    it("reads the live view of the board, so a column edit shows when its frame arrives", () => {
        // when
        renderPage();

        // then
        expect(useBoardView).toHaveBeenCalledWith(BOARD_ID);
    });

    it("lets a facilitator change the description and every column, with a way back to the board", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Board settings" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to board" })).toHaveAttribute(
            "href",
            `/o/harbour/boards/${BOARD_ID}`,
        );
        expect(screen.getByRole("button", { name: "Edit description" })).toBeInTheDocument();
        const columns = screen.getByRole("list", { name: "Columns" });
        expect(within(columns).getAllByRole("listitem")).toHaveLength(3);
        expect(screen.getByRole("button", { name: "Options for Start" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Add a column" })).toBeInTheDocument();
    });

    it("gives a facilitator the voting and card switches beside the columns", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("region", { name: "Voting" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Voting open" })).toHaveAttribute("aria-checked", "true");
        expect(screen.getByRole("region", { name: "Cards" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Merging" })).toBeInTheDocument();
    });

    it("tells a participant only facilitators change the settings", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Only the board's facilitators can change its settings.")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a column" })).not.toBeInTheDocument();
        expect(screen.queryByRole("switch", { name: "Voting open" })).not.toBeInTheDocument();
    });

    it("says an archived board's settings can no longer change", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }) }));

        // when
        renderPage();

        // then
        expect(screen.getByText("This board is archived, so its settings can no longer change.")).toBeInTheDocument();
    });

    it("still lets a manager unarchive, move or delete an archived board", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }),
                viewer: makeBoardViewer({ capabilities: ["board.view", "board.manage", "board.export"] }),
            }),
        );

        // when
        renderPage();

        // then
        const manage = screen.getByRole("region", { name: "Manage board" });
        expect(within(manage).getByRole("button", { name: "Unarchive board" })).toBeInTheDocument();
        expect(within(manage).getByRole("button", { name: "Delete board" })).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Import cards" })).not.toBeInTheDocument();
    });

    it("gives a facilitator the card import, the background and the board's management in the Write stage", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("region", { name: "Import cards" })).toBeInTheDocument();
        expect(BoardBackgroundSection.mock.calls.at(-1)?.[0]).toMatchObject({ snapshot: { board_id: BOARD_ID } });
        expect(screen.getByRole("region", { name: "Manage board" })).toBeInTheDocument();
    });

    it("offers no card import outside the Write stage", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ stage: "vote" }) }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("region", { name: "Import cards" })).not.toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Manage board" })).toBeInTheDocument();
    });

    it("offers a participant none of the board's management", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("region", { name: "Manage board" })).not.toBeInTheDocument();
    });

    it("says an ended retro's settings can no longer change", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ closed: true, cards_hidden: false }) }));

        // when
        renderPage();

        // then
        expect(screen.getByText("This retro has ended, so its settings can no longer change.")).toBeInTheDocument();
        expect(screen.queryByRole("switch", { name: "Lock the board" })).not.toBeInTheDocument();
    });

    it("gives a facilitator the locks, the timer sound and the people on the board", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                participants: [
                    makeBoardParticipant(),
                    makeBoardParticipant({ participant_id: "p2", display_name: "Ali", role: "participant" }),
                ],
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("switch", { name: "Cards hidden" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Lock cards" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Lock the board" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Timer sound" })).toBeInTheDocument();
        const people = screen.getByRole("region", { name: "People" });
        expect(within(people).getByRole("button", { name: "Actions for Ali" })).toBeInTheDocument();
    });
});
