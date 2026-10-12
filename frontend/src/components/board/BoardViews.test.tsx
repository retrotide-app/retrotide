import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CardFilter } from "../../domain/board/cardFilter";
import type { BoardLayout } from "../../hooks/board/useBoardLayout";
import { createCardFocus } from "../../hooks/board/useCardFocus";
import { makeBoardSettings, makeBoardSnapshot } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardViews } from "./BoardViews";

const { useBoardLayout } = vi.hoisted(() => ({ useBoardLayout: vi.fn() }));

vi.mock("../../hooks/board/useBoardLayout", () => ({ useBoardLayout }));
vi.mock("./BoardColumns", () => ({
    BoardColumns: ({
        snapshot,
        writable,
        filter,
    }: {
        snapshot: BoardSnapshot;
        writable: boolean;
        filter: CardFilter;
    }) => (
        <>
            <p>
                columns of {snapshot.board.title}, writable {String(writable)}
            </p>
            <p>{`searching "${filter.query}", sorted by ${filter.sort}`}</p>
        </>
    ),
}));
vi.mock("./list/BoardListView", () => ({
    BoardListView: ({ snapshot, writable }: { snapshot: BoardSnapshot; writable: boolean }) => (
        <p>
            list of {snapshot.board.title}, writable {String(writable)}
        </p>
    ),
}));

const choose = vi.fn();

function showLayout(layout: BoardLayout) {
    useBoardLayout.mockReturnValue({ layout, choose });
}

beforeEach(() => {
    vi.clearAllMocks();
    showLayout("columns");
});

describe("BoardViews", () => {
    it("shows the column view when the layout is columns", () => {
        // when
        renderWithProviders(<BoardViews snapshot={makeBoardSnapshot()} writable cardFocus={createCardFocus()} />);

        // then
        expect(screen.getByText("columns of Sprint 12, writable true")).toBeInTheDocument();
        expect(screen.queryByText(/^list of/)).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Columns", pressed: true })).toBeInTheDocument();
    });

    it("shows the list view when the layout is list", () => {
        // given
        showLayout("list");

        // when
        renderWithProviders(
            <BoardViews snapshot={makeBoardSnapshot()} writable={false} cardFocus={createCardFocus()} />,
        );

        // then
        expect(screen.getByText("list of Sprint 12, writable false")).toBeInTheDocument();
        expect(screen.queryByText(/^columns of/)).not.toBeInTheDocument();
    });

    it("switches layout when the viewer picks one", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<BoardViews snapshot={makeBoardSnapshot()} writable cardFocus={createCardFocus()} />);

        // when
        await user.click(screen.getByRole("button", { name: "List" }));

        // then
        expect(choose).toHaveBeenCalledExactlyOnceWith("list");
    });

    it("starts in the facilitator's order and hands the viewer's search and sort to the board", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <BoardViews
                snapshot={makeBoardSnapshot({ settings: makeBoardSettings({ sort_mode: "votes" }) })}
                writable
                cardFocus={createCardFocus()}
            />,
        );
        const started = screen.getByText(/^searching/).textContent;

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search cards" }), "deploy");
        await user.selectOptions(screen.getByRole("combobox", { name: "Sort cards" }), "Newest first");

        // then
        expect(started).toBe('searching "", sorted by votes');
        expect(screen.getByText('searching "deploy", sorted by date')).toBeInTheDocument();
    });

    it("holds the board to the facilitator's order while it is enforced, and follows a change to it", async () => {
        // given
        const user = userEvent.setup();
        const enforced = (sortMode: "manual" | "votes") =>
            makeBoardSnapshot({ settings: makeBoardSettings({ sort_mode: sortMode, sort_enforced: true }) });
        const { rerender } = renderWithProviders(
            <BoardViews snapshot={enforced("votes")} writable cardFocus={createCardFocus()} />,
        );

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search cards" }), "deploy");
        const searched = screen.getByText(/^searching/).textContent;
        rerender(<BoardViews snapshot={enforced("manual")} writable cardFocus={createCardFocus()} />);

        // then
        expect(searched).toBe('searching "deploy", sorted by votes');
        expect(screen.getByText('searching "deploy", sorted by manual')).toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Sort cards" })).toBeDisabled();
    });
});
