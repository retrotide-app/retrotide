import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UNFILTERED } from "../../domain/board/cardFilter";
import { BOARD_LAYOUT_ID, BOARD_SEARCH_ID } from "../../domain/board/shortcuts";
import type { CardFilterState } from "../../hooks/board/useCardFilter";
import {
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeMe,
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardToolbar } from "./BoardToolbar";

const sam = { participant_id: "p-sam", display_name: "Sam" };
const ali = { participant_id: "p-ali", display_name: "Ali" };

const cards: CardFilterState = {
    filter: UNFILTERED,
    search: vi.fn(),
    sortBy: vi.fn(),
    showAuthor: vi.fn(),
};

const onLayoutChange = vi.fn();

const voting = makeBoardView({ stage: "vote", cards_hidden: false });

function renderToolbar(snapshot: BoardSnapshot, writable = true, state: CardFilterState = cards) {
    return renderWithProviders(
        <BoardToolbar
            snapshot={snapshot}
            writable={writable}
            cards={state}
            layout="columns"
            onLayoutChange={onLayoutChange}
        />,
    );
}

function optionsOf(name: string): string[] {
    return within(screen.getByRole("combobox", { name }))
        .getAllByRole("option")
        .map(option => option.textContent ?? "");
}

beforeEach(() => {
    vi.clearAllMocks();
});

describe("BoardToolbar", () => {
    it("searches the cards as the viewer types", async () => {
        // given
        const user = userEvent.setup();
        renderToolbar(makeBoardSnapshot());

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search cards" }), "d");

        // then
        expect(cards.search).toHaveBeenCalledWith("d");
    });

    it("offers every order on a named board and sorts by the one picked", async () => {
        // given
        const user = userEvent.setup();
        renderToolbar(makeBoardSnapshot());

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Sort cards" }), "Most votes");

        // then
        expect(optionsOf("Sort cards")).toEqual(["Board order", "Most votes", "Newest first", "Shuffled"]);
        expect(cards.sortBy).toHaveBeenCalledExactlyOnceWith("votes");
    });

    it("filters by an author picked from the people who wrote cards, and back to everyone", async () => {
        // given
        const user = userEvent.setup();
        const snapshot = makeBoardSnapshot({
            cards: [makeCard({ id: "k1", author: sam }), makeCard({ id: "k2", author: ali })],
        });
        renderToolbar(snapshot);

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Filter by author" }), "Ali");
        await user.selectOptions(screen.getByRole("combobox", { name: "Filter by author" }), "Everyone");

        // then
        expect(optionsOf("Filter by author")).toEqual(["Everyone", "Ali", "Sam"]);
        expect(cards.showAuthor).toHaveBeenNthCalledWith(1, "p-ali");
        expect(cards.showAuthor).toHaveBeenNthCalledWith(2, null);
    });

    it("never offers an author filter or a date order on an anonymous board", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ anonymity: "anonymous" }),
            cards: [makeCard({ author: null })],
        });

        // when
        renderToolbar(snapshot);

        // then
        expect(screen.queryByRole("combobox", { name: "Filter by author" })).not.toBeInTheDocument();
        expect(optionsOf("Sort cards")).not.toContain("Newest first");
    });

    it("shows how many votes the viewer has left in Vote while voting is open", () => {
        // when
        renderToolbar(
            makeBoardSnapshot({
                board: voting,
                viewer: makeParticipantViewer(),
                me: makeMe({ votes_remaining: 4 }),
            }),
        );

        // then
        expect(screen.getByRole("status")).toHaveTextContent("4 of 6 votes left");
    });

    it.each(["write", "discuss", "actions"] as const)("leaves the votes left out in %s", stage => {
        // when
        renderToolbar(
            makeBoardSnapshot({
                board: makeBoardView({ stage, cards_hidden: stage === "write" }),
                viewer: makeParticipantViewer(),
            }),
        );

        // then
        expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it("leaves the votes out while voting is closed or before the viewer has joined", () => {
        // when
        const { unmount } = renderToolbar(
            makeBoardSnapshot({ board: voting, settings: makeBoardSettings({ voting_open: false }) }),
        );
        const closed = screen.queryByRole("status");
        unmount();
        renderToolbar(makeBoardSnapshot({ board: voting }), false);

        // then
        expect(closed).not.toBeInTheDocument();
        expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it("holds the sort while the facilitator enforces it, saying so, and keeps search and the author filter", async () => {
        // given
        const user = userEvent.setup();
        const snapshot = makeBoardSnapshot({
            settings: makeBoardSettings({ sort_enforced: true, sort_mode: "votes" }),
            cards: [makeCard({ id: "k1", author: sam }), makeCard({ id: "k2", author: ali })],
        });
        renderToolbar(snapshot, true, { ...cards, filter: { ...UNFILTERED, sort: "votes" } });

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search cards" }), "d");
        await user.selectOptions(screen.getByRole("combobox", { name: "Filter by author" }), "Ali");

        // then
        const sort = screen.getByRole("combobox", { name: "Sort cards" });
        expect(sort).toBeDisabled();
        expect(sort).toHaveValue("votes");
        expect(sort).toHaveAccessibleDescription("Sort order set by the facilitator");
        expect(screen.getByText("Sort order set by the facilitator")).toBeVisible();
        expect(cards.search).toHaveBeenCalledWith("d");
        expect(cards.showAuthor).toHaveBeenCalledWith("p-ali");
    });

    it("leaves the sort open with no note while the order is not enforced", () => {
        // when
        renderToolbar(makeBoardSnapshot());

        // then
        expect(screen.getByRole("combobox", { name: "Sort cards" })).toBeEnabled();
        expect(screen.queryByText("Sort order set by the facilitator")).not.toBeInTheDocument();
    });

    it("switches the layout from the toolbar", async () => {
        // given
        const user = userEvent.setup();
        renderToolbar(makeBoardSnapshot());

        // when
        await user.click(screen.getByRole("button", { name: "List" }));

        // then
        expect(onLayoutChange).toHaveBeenCalledExactlyOnceWith("list");
    });

    it("marks the search and the layout switch for the keyboard shortcuts", () => {
        // when
        renderToolbar(makeBoardSnapshot());

        // then
        expect(screen.getByRole("searchbox", { name: "Search cards" })).toHaveAttribute("id", BOARD_SEARCH_ID);
        expect(document.getElementById(BOARD_LAYOUT_ID)).toContainElement(screen.getByRole("button", { name: "List" }));
    });
});
