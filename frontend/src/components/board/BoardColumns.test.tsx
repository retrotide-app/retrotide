import type * as DndKit from "@dnd-kit/react";
import type { DragEndEvent } from "@dnd-kit/react";
import { screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UNFILTERED } from "../../domain/board/cardFilter";
import type { BoardDragData } from "../../domain/board/drop";
import { createCardFocus } from "../../hooks/board/useCardFocus";
import {
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeMutationState,
    makeParticipantViewer,
    makeSpotlight,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardColumns } from "./BoardColumns";

const dragEnd = vi.hoisted(() => ({ handler: vi.fn() }));

vi.mock("@dnd-kit/react", async importOriginal => {
    const actual = await importOriginal<typeof DndKit>();

    return {
        ...actual,
        DragDropProvider: (props: ComponentProps<typeof actual.DragDropProvider>) => {
            dragEnd.handler.mockImplementation((event: DragEndEvent) => props.onDragEnd?.(event, undefined as never));

            return <actual.DragDropProvider {...props} />;
        },
    };
});

const mutations = vi.hoisted(() => ({
    useCreateCard: vi.fn(),
    useEditCard: vi.fn(),
    useRecolourCard: vi.fn(),
    useDeleteCard: vi.fn(),
    useMoveCard: vi.fn(),
    useCastVote: vi.fn(),
    useRemoveVote: vi.fn(),
    useMergeCard: vi.fn(),
    useUnmergeCard: vi.fn(),
    useCreateComment: vi.fn(),
    useEditComment: vi.fn(),
    useDeleteComment: vi.fn(),
    useUpdateColumn: vi.fn(),
    useMoveColumn: vi.fn(),
    useCreateColumn: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => mutations);

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const CONTINUE = "00000000-0000-4000-8000-000000000c03";

const draggable = makeBoardSnapshot({
    cards: [
        makeCard({ id: "k1", column_id: START, position: "a0", body: "First" }),
        makeCard({ id: "k2", column_id: START, position: "a1", body: "Second" }),
        makeCard({ id: "k9", column_id: STOP, position: "a0", body: "Elsewhere" }),
    ],
});

function renderColumns(snapshot: BoardSnapshot = makeBoardSnapshot(), writable = true) {
    return renderWithProviders(<BoardColumns snapshot={snapshot} writable={writable} cardFocus={createCardFocus()} />);
}

function endDrag(source: BoardDragData | null, target: BoardDragData | null, canceled = false) {
    const event = {
        operation: {
            source: source === null ? null : { data: source },
            target: target === null ? null : { data: target },
        },
        canceled,
    };

    dragEnd.handler(event as unknown as DragEndEvent);
}

let move = makeMutationState();

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    move = makeMutationState();
    mutations.useMoveCard.mockReturnValue(move);
});

describe("BoardColumns", () => {
    it("lays the columns out in position order", () => {
        // given
        const snapshot = makeBoardSnapshot();
        const reversed = { ...snapshot, columns: snapshot.columns.toReversed() };

        // when
        renderColumns(reversed);

        // then
        const headings = screen.getAllByRole("heading", { level: 2 });
        expect(headings.map(heading => heading.textContent)).toEqual(["Start", "Stop", "Continue"]);
    });

    it("offers a facilitator an Add a column tile at the end of the grid", () => {
        // when
        renderColumns();

        // then
        expect(screen.getByRole("button", { name: "Add a column" })).toBeInTheDocument();
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
    });

    it("offers a seated participant the Add a column tile only while anyone can edit", () => {
        // given
        const participant = makeParticipantViewer();
        const { unmount } = renderColumns(makeBoardSnapshot({ viewer: participant }));
        const withoutEditing = screen.queryByRole("button", { name: "Add a column" });
        unmount();

        // when
        renderColumns(
            makeBoardSnapshot({ viewer: participant, settings: makeBoardSettings({ anyone_can_edit: true }) }),
        );

        // then
        expect(withoutEditing).toBeNull();
        expect(screen.getByRole("button", { name: "Add a column" })).toBeInTheDocument();
    });

    it("offers no Add a column tile before the viewer may write", () => {
        // when
        renderColumns(makeBoardSnapshot(), false);

        // then
        expect(screen.queryByRole("button", { name: "Add a column" })).not.toBeInTheDocument();
    });

    it("shows the focused column wide and collapses the others to their titles", () => {
        // given
        const snapshot = makeBoardSnapshot({ spotlight: makeSpotlight({ focused_column_id: STOP }) });

        // when
        renderColumns(snapshot);

        // then the collapsed columns keep their titles but hold no cards or controls
        const headings = screen.getAllByRole("heading", { level: 2 });
        expect(headings.map(heading => heading.textContent)).toEqual(["Start", "Stop", "Continue"]);
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(1);
        expect(screen.queryByRole("article", { name: "Pair on reviews" })).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Focused on Stop");
        expect(screen.queryByRole("button", { name: "Add a column" })).not.toBeInTheDocument();
    });

    it("lays every column out as usual when the focused column is not on the board", () => {
        // given
        const snapshot = makeBoardSnapshot({ spotlight: makeSpotlight({ focused_column_id: "gone" }) });

        // when
        renderColumns(snapshot);

        // then
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
        expect(screen.getByRole("status")).toBeEmptyDOMElement();
    });

    it("gives a facilitator and a participant alike a menu on every column, for copying it at least", () => {
        // when
        const { unmount } = renderColumns();
        const facilitated = screen.getAllByRole("button", { name: /^Options for/ }).length;
        unmount();
        renderColumns(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // then
        expect(facilitated).toBe(3);
        expect(screen.getAllByRole("button", { name: /^Options for/ })).toHaveLength(3);
    });

    it("lets a participant add cards", () => {
        // when
        renderColumns(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // then
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
    });

    it("shows no write controls until the viewer may write", () => {
        // when
        renderColumns(makeBoardSnapshot(), false);

        // then
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
        expect(screen.getByRole("article", { name: "Pair on reviews" })).toBeInTheDocument();
    });

    it("shows no write controls on an archived board", () => {
        // when
        renderColumns(makeBoardSnapshot({ board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }) }));

        // then
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
    });

    it("says when the board has no columns", () => {
        // when
        renderColumns(makeBoardSnapshot({ columns: [], cards: [], hidden_counts: {} }));

        // then
        expect(screen.getByText("This board has no columns yet.")).toBeInTheDocument();
    });

    it("shows a refused drag move above the columns", () => {
        // given
        mutations.useMoveCard.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "moves are locked", code: "moves_locked" }),
            }),
        );

        // when
        renderColumns();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("moves are locked");
    });

    it("moves a card dropped on a card in another column before that card", () => {
        // given
        renderColumns(draggable);

        // when
        endDrag({ kind: "card", id: "k1" }, { kind: "card", id: "k9" });

        // then
        expect(move.mutate).toHaveBeenCalledExactlyOnceWith({
            cardId: "k1",
            column_id: STOP,
            after_card_id: null,
        });
    });

    it("moves a card dropped on a column's card area to the bottom of that column", () => {
        // given
        renderColumns(draggable);

        // when
        endDrag({ kind: "card", id: "k9" }, { kind: "column-cards", id: START });

        // then
        expect(move.mutate).toHaveBeenCalledExactlyOnceWith({
            cardId: "k9",
            column_id: START,
            after_card_id: "k2",
        });
    });

    it("moves nothing for a cancelled drag, a drop on nothing, a drop back in place or a column", () => {
        // given
        renderColumns(draggable);

        // when
        endDrag({ kind: "card", id: "k1" }, { kind: "card", id: "k9" }, true);
        endDrag({ kind: "card", id: "k1" }, null);
        endDrag({ kind: "card", id: "k2" }, { kind: "column-cards", id: START });
        endDrag({ kind: "column", id: START }, { kind: "column", id: CONTINUE });

        // then
        expect(move.mutate).not.toHaveBeenCalled();
    });

    it.each(["vote", "discuss", "actions"] as const)(
        "hides Add a card and moves nothing in %s, because cards are written and moved only in Write",
        stage => {
            // given
            renderColumns({ ...draggable, board: makeBoardView({ stage, cards_hidden: false }) });

            // when
            endDrag({ kind: "card", id: "k1" }, { kind: "card", id: "k9" });

            // then
            expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
            expect(move.mutate).not.toHaveBeenCalled();
        },
    );

    it("merges a card dropped on another card's merge target in Discuss", () => {
        // given
        const merge = makeMutationState();
        mutations.useMergeCard.mockReturnValue(merge);
        renderColumns({ ...draggable, board: makeBoardView({ stage: "discuss", cards_hidden: false }) });

        // when
        endDrag({ kind: "card", id: "k1" }, { kind: "card-merge", id: "k9" });

        // then
        expect(merge.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1", targetCardId: "k9" });
        expect(move.mutate).not.toHaveBeenCalled();
    });

    it("merges nothing in Write or Vote, or while merging is switched off", () => {
        // given
        const merge = makeMutationState();
        mutations.useMergeCard.mockReturnValue(merge);
        const { unmount } = renderColumns(draggable);
        endDrag({ kind: "card", id: "k1" }, { kind: "card-merge", id: "k9" });
        unmount();
        const voting = renderColumns({ ...draggable, board: makeBoardView({ stage: "vote", cards_hidden: false }) });
        endDrag({ kind: "card", id: "k1" }, { kind: "card-merge", id: "k9" });
        voting.unmount();

        // when
        renderColumns({
            ...draggable,
            board: makeBoardView({ stage: "discuss", cards_hidden: false }),
            settings: makeBoardSettings({ merging_enabled: false }),
        });
        endDrag({ kind: "card", id: "k1" }, { kind: "card-merge", id: "k9" });

        // then
        expect(merge.mutate).not.toHaveBeenCalled();
    });

    it("never reorders cards while the view is sorted or searched", () => {
        // given
        renderWithProviders(
            <BoardColumns
                snapshot={draggable}
                writable
                cardFocus={createCardFocus()}
                filter={{ ...UNFILTERED, sort: "votes" }}
            />,
        );

        // when
        endDrag({ kind: "card", id: "k1" }, { kind: "card", id: "k9" });

        // then
        expect(move.mutate).not.toHaveBeenCalled();
    });

    it("shows only the cards that match the search", () => {
        // when
        renderWithProviders(
            <BoardColumns
                snapshot={draggable}
                writable
                cardFocus={createCardFocus()}
                filter={{ ...UNFILTERED, query: "first" }}
            />,
        );

        // then
        expect(screen.getByRole("article", { name: "First" })).toBeInTheDocument();
        expect(screen.queryByRole("article", { name: "Second" })).not.toBeInTheDocument();
        expect(screen.getAllByText("No matching cards.")).toHaveLength(2);
    });
});
