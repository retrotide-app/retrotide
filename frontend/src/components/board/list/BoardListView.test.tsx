import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCardFocus } from "../../../hooks/board/useCardFocus";
import {
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeMutationState,
    makeParticipantViewer,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BoardSnapshot } from "../../../types/api";
import { BoardListView } from "./BoardListView";

const hooks = vi.hoisted(() => ({
    useCreateCard: vi.fn(),
    useEditCard: vi.fn(),
    useRecolourCard: vi.fn(),
    useMoveCard: vi.fn(),
    useDeleteCard: vi.fn(),
    useCreateColumn: vi.fn(),
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

vi.mock("../../../hooks/mutations/boards", () => hooks);

const CONTINUE = "00000000-0000-4000-8000-000000000c03";

function renderList(snapshot: BoardSnapshot = makeBoardSnapshot(), writable = true) {
    return renderWithProviders(<BoardListView snapshot={snapshot} writable={writable} cardFocus={createCardFocus()} />);
}

function section(name: string): HTMLElement {
    return screen.getByRole("region", { name });
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(hooks)) {
        hook.mockReturnValue(makeMutationState());
    }
});

describe("BoardListView", () => {
    it("stacks one section per column, in column order", () => {
        // when
        renderList();

        // then
        const headings = screen.getAllByRole("heading", { level: 2 }).map(heading => heading.textContent);
        expect(headings).toEqual(["Start", "Stop", "Continue"]);
        expect(section("Start")).toHaveTextContent("What should we begin doing?");
    });

    it("shows the viewer's own cards and counts everyone else's hidden cards without their text", () => {
        // when
        renderList();

        // then
        expect(within(section("Start")).getByRole("list", { name: "Cards in Start" })).toHaveTextContent(
            "Pair on reviews",
        );
        expect(section("Start")).toHaveTextContent("1 card");
        expect(section("Stop")).toHaveTextContent("2 cards");
        expect(section("Stop")).toHaveTextContent("2 cards from others, hidden for now");
        expect(within(section("Stop")).queryByRole("list", { name: "Cards in Stop" })).not.toBeInTheDocument();
        expect(section("Continue")).toHaveTextContent("No cards yet.");
    });

    it("lists a column's cards in position order", () => {
        // given
        const snapshot = makeBoardSnapshot({
            cards: [
                makeCard({
                    id: "00000000-0000-4000-8000-000000000a02",
                    column_id: CONTINUE,
                    position: "a2",
                    body: "Second",
                }),
                makeCard({
                    id: "00000000-0000-4000-8000-000000000a03",
                    column_id: CONTINUE,
                    position: "a1",
                    body: "First",
                }),
            ],
        });

        // when
        renderList(snapshot);

        // then
        const list = within(section("Continue")).getByRole("list", { name: "Cards in Continue" });
        const items = within(list).getAllByRole("listitem");
        expect(items).toHaveLength(2);
        expect(items[0]).toHaveTextContent("First");
        expect(items[1]).toHaveTextContent("Second");
    });

    it("lets a participant add a card to every column and move a card with a menu, never a drag", async () => {
        // given
        const user = userEvent.setup();
        renderList(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // when
        await user.click(within(section("Start")).getAllByRole("button", { name: "Card options" })[0]);
        await user.click(screen.getByRole("menuitem", { name: "Move card" }));

        // then
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
        expect(screen.getByRole("combobox", { name: "Move to" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: /^Drag/ })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a column" })).not.toBeInTheDocument();
    });

    it("leaves setting up the columns to board settings, even for a facilitator", () => {
        // when
        renderList();

        // then
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
        expect(screen.queryByRole("button", { name: "Options for Start" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a column" })).not.toBeInTheDocument();
    });

    it("offers nothing but copying until the viewer has joined the board", async () => {
        // given
        const user = userEvent.setup();

        // when
        renderList(makeBoardSnapshot(), false);

        // then
        await user.click(screen.getByRole("button", { name: "Card options" }));
        expect(
            within(screen.getByRole("menu"))
                .getAllByRole("menuitem")
                .map(item => item.textContent),
        ).toEqual(["Copy text"]);
        expect(screen.getByText("Pair on reviews")).toBeInTheDocument();
    });

    it("offers nothing but copying on an archived board", async () => {
        // given
        const user = userEvent.setup();

        // when
        renderList(makeBoardSnapshot({ board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }) }));

        // then
        await user.click(screen.getByRole("button", { name: "Card options" }));
        expect(
            within(screen.getByRole("menu"))
                .getAllByRole("menuitem")
                .map(item => item.textContent),
        ).toEqual(["Copy text"]);
        expect(screen.getByText("Pair on reviews")).toBeInTheDocument();
    });

    it("says when the board has no columns", () => {
        // when
        renderList(makeBoardSnapshot({ columns: [], cards: [], hidden_counts: {} }));

        // then
        expect(screen.getByText("This board has no columns yet.")).toBeInTheDocument();
    });
});
