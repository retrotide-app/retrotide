import { DragDropProvider } from "@dnd-kit/react";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardActions } from "../../domain/board/capabilities";
import { boardColumns } from "../../domain/board/selectors";
import { createCardFocus } from "../../hooks/board/useCardFocus";
import {
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeColumn,
    makeMe,
    makeMutationState,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardColumn } from "./BoardColumn";

const mutations = vi.hoisted(() => ({
    useCreateCard: vi.fn(),
    useEditCard: vi.fn(),
    useRecolourCard: vi.fn(),
    useDeleteCard: vi.fn(),
    useMoveCard: vi.fn(),
    useUpdateColumn: vi.fn(),
    useMoveColumn: vi.fn(),
    useClearColumn: vi.fn(),
    useDeleteColumn: vi.fn(),
    useCastVote: vi.fn(),
    useRemoveVote: vi.fn(),
    useMergeCard: vi.fn(),
    useUnmergeCard: vi.fn(),
    useCreateComment: vi.fn(),
    useEditComment: vi.fn(),
    useDeleteComment: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => mutations);

const facilitation = vi.hoisted(() => ({
    useRevealColumn: vi.fn(),
    useHideColumn: vi.fn(),
    useShowColumn: vi.fn(),
    useResetColumnVotes: vi.fn(),
    useSetColumnVoteLimit: vi.fn(),
    usePresentCard: vi.fn(),
    useHighlightCard: vi.fn(),
    useFocusColumn: vi.fn(),
}));

vi.mock("../../hooks/mutations/facilitation", () => facilitation);

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const EVERYTHING: BoardActions = {
    addCards: true,
    moveCards: true,
    manageColumns: true,
    removeAnyCard: true,
    describeBoard: true,
    removeColumns: true,
    controlTimer: true,
    spotlight: true,
};

const NOTHING: BoardActions = {
    addCards: false,
    moveCards: false,
    manageColumns: false,
    removeAnyCard: false,
    describeBoard: false,
    removeColumns: false,
    controlTimer: false,
    spotlight: false,
};

function renderColumn(snapshot: BoardSnapshot, columnId: string, actions: BoardActions = EVERYTHING) {
    const models = boardColumns(snapshot);
    const index = models.findIndex(model => model.column.id === columnId);

    return renderWithProviders(
        <DragDropProvider>
            <BoardColumn
                snapshot={snapshot}
                model={models[index]}
                actions={actions}
                writable
                cardFocus={createCardFocus()}
            />
        </DragDropProvider>,
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of [...Object.values(mutations), ...Object.values(facilitation)]) {
        hook.mockReturnValue(makeMutationState());
    }
});

describe("BoardColumn", () => {
    it("shows the column's title, description, count and cards in position order", () => {
        // given
        const snapshot = makeBoardSnapshot({
            cards: [
                makeCard({ id: "k2", column_id: START, position: "a1", body: "Second" }),
                makeCard({ id: "k1", column_id: START, position: "a0", body: "First" }),
            ],
            hidden_counts: {},
        });

        // when
        renderColumn(snapshot, START);

        // then
        const column = screen.getByRole("region", { name: "Start" });
        expect(within(column).getByText("What should we begin doing?")).toBeInTheDocument();
        expect(within(column).getByText("2 cards")).toBeInTheDocument();
        const cards = within(within(column).getByRole("list", { name: "Cards in Start" })).getAllByRole("article");
        expect(cards.map(card => card.textContent)).toEqual([
            expect.stringContaining("First"),
            expect.stringContaining("Second"),
        ]);
    });

    it("draws other people's hidden cards as blurred placeholders and counts them", () => {
        // given
        const snapshot = makeBoardSnapshot();

        // when
        renderColumn(snapshot, STOP);

        // then
        const column = screen.getByRole("region", { name: "Stop" });
        expect(within(column).getByText("2 cards")).toBeInTheDocument();
        expect(within(column).getByText("2 cards from others, hidden for now")).toBeInTheDocument();
        expect(within(column).queryByRole("article")).not.toBeInTheDocument();
    });

    it("says when a column has no cards", () => {
        // given
        const snapshot = makeBoardSnapshot({ cards: [], hidden_counts: {} });

        // when
        renderColumn(snapshot, START);

        // then
        expect(screen.getByText("No cards yet.")).toBeInTheDocument();
    });

    it("gives a facilitator the column's menu and a way to add cards", async () => {
        // given
        const user = userEvent.setup();
        renderColumn(makeBoardSnapshot(), START);
        const column = screen.getByRole("region", { name: "Start" });

        // when
        await user.click(within(column).getByRole("button", { name: "Options for Start" }));

        // then
        const items = within(screen.getByRole("menu")).getAllByRole("menuitem");
        expect(items.map(item => item.textContent)).toEqual([
            "Edit column",
            "Move right",
            "Reveal cards",
            "Hide from participants",
            "Vote limit…",
            "Copy column",
            "Reset votes",
        ]);
        expect(within(column).queryByRole("button", { name: "Drag column" })).not.toBeInTheDocument();
        expect(within(column).getByRole("button", { name: "Add a card" })).toBeInTheDocument();
    });

    it("shows a facilitator a hidden column only as a notice until they show it for themselves", async () => {
        // given a column hidden from participants, holding the facilitator's own card
        const user = userEvent.setup();
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn({ hidden: true })],
            cards: [makeCard({ column_id: START, body: "Pair on reviews" })],
            hidden_counts: {},
        });
        renderColumn(snapshot, START);
        const column = screen.getByRole("region", { name: "Start" });

        // when
        const before = within(column).queryByText("Pair on reviews");
        await user.click(within(column).getByRole("button", { name: "Show for me" }));

        // then the cards appear, read-only and with no way to add one
        expect(before).not.toBeInTheDocument();
        expect(within(column).getByText("Pair on reviews")).toBeInTheDocument();
        expect(within(column).queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
        expect(within(column).getByText("Hidden from participants")).toBeInTheDocument();

        // when
        await user.click(within(column).getByRole("button", { name: "Hide again for me" }));

        // then
        expect(within(column).queryByText("Pair on reviews")).not.toBeInTheDocument();
        expect(within(column).getByRole("button", { name: "Show for me" })).toBeInTheDocument();
    });

    it("shows a hidden column to everyone from its notice", async () => {
        // given
        const user = userEvent.setup();
        const show = makeMutationState();
        facilitation.useShowColumn.mockReturnValue(show);
        renderColumn(makeBoardSnapshot({ columns: [makeColumn({ hidden: true })] }), START);

        // when
        await user.click(screen.getByRole("button", { name: "Show to everyone" }));

        // then
        expect(show.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: START });
    });

    it("draws no hidden placeholders in a column whose cards were revealed", () => {
        // given a stray hidden count for the revealed column
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn(), makeColumn({ id: STOP, title: "Stop", position: "a1", cards_revealed: true })],
        });

        // when
        renderColumn(snapshot, STOP);

        // then
        const column = screen.getByRole("region", { name: "Stop" });
        expect(within(column).queryByText("2 cards from others, hidden for now")).not.toBeInTheDocument();
        expect(within(column).getByText("No cards yet.")).toBeInTheDocument();
    });

    it("tells a voter how many of the column's votes they have left in the Vote stage", () => {
        // given a column allowing three votes, one of them already on its card
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ stage: "vote", cards_hidden: false }),
            columns: [makeColumn({ max_votes: 3 })],
            cards: [makeCard({ id: "k1", column_id: START, hidden: false })],
            me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }),
            hidden_counts: {},
        });

        // when
        renderColumn(snapshot, START, NOTHING);

        // then
        expect(screen.getByText("2 of 3 votes left in this column")).toBeInTheDocument();
    });

    it("says nothing of a column's votes outside the Vote stage", () => {
        // given
        const snapshot = makeBoardSnapshot({ columns: [makeColumn({ max_votes: 3 })] });

        // when
        renderColumn(snapshot, START, NOTHING);

        // then
        expect(screen.queryByText(/votes left in this column/)).not.toBeInTheDocument();
    });

    it("gives a participant a way to add cards but no column tools beyond copying", () => {
        // when
        renderColumn(makeBoardSnapshot(), START, { ...NOTHING, addCards: true, moveCards: true });

        // then
        const column = screen.getByRole("region", { name: "Start" });
        expect(within(column).getByRole("button", { name: "Options for Start" })).toBeInTheDocument();
        expect(within(column).queryByRole("button", { name: "Drag column" })).not.toBeInTheDocument();
        expect(within(column).getByRole("button", { name: "Add a card" })).toBeInTheDocument();
    });

    it("leaves out the description line when the column has none", () => {
        // given
        const snapshot = makeBoardSnapshot({ columns: [makeColumn({ description: "" })] });

        // when
        renderColumn(snapshot, START, NOTHING);

        // then
        expect(screen.queryByText("What should we begin doing?")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
    });
});
