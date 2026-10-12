import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { QueryState } from "../../hooks/queries/queryState";
import {
    BOARD_ID,
    makeActionItem,
    makeApiError,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeComment,
    makeOrganisation,
    makeParticipantViewer,
    makePerson,
    makeQueryState,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, CardView } from "../../types/api";
import { BoardSummaryPage } from "./BoardSummaryPage";

const { useBoard } = vi.hoisted(() => ({ useBoard: vi.fn() }));

vi.mock("../../hooks/queries/boards", () => ({ useBoard }));

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const harbour = makeOrganisation({ slug: "harbour" });

const sam = makePerson({ user_id: "u-sam", display_name: "Sam" });

function card(id: string, body: string, votes: number, overrides: Partial<CardView> = {}): CardView {
    return makeCard({ id, body, column_id: START, hidden: false, votes: { total: votes }, ...overrides });
}

function showBoard(state: Partial<QueryState<BoardSnapshot>>) {
    const query = makeQueryState<BoardSnapshot>(state);
    useBoard.mockReturnValue(query);

    return query;
}

function renderPage(deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, deleted_at: deletedAt });

    return renderWithProviders(<BoardSummaryPage />, {
        org: { organisations: [current], current },
        route: `/o/harbour/boards/${BOARD_ID}/summary`,
        path: "/o/:orgSlug/boards/:boardId/summary",
    });
}

const discussed = makeBoardSnapshot({
    board: makeBoardView({ stage: "discuss", cards_hidden: false }),
    hidden_counts: { [STOP]: 2 },
    cards: [
        card("k1", "Standups ran long", 1),
        card("k2", "Reviews were slow", 3),
        card("k3", "Reviews waited on one person", 2, { parent_card_id: "k2" }),
    ],
    comments: [makeComment({ id: "m1", card_id: "k2", body: "Pair on them" })],
    action_items: [
        makeActionItem({
            id: "i1",
            title: "Ask @Sam to rotate reviewers",
            mentions: [sam],
            assignee: sam,
            due_on: "2026-10-15",
            source_card_id: "k2",
        }),
        makeActionItem({ id: "i2", title: "Timebox standups", status: "done", source_card_id: "k1" }),
        makeActionItem({ id: "i3", title: "Book the retro room", source_card_id: null }),
        makeActionItem({ id: "i4", title: "From a hidden card", source_card_id: "someone-elses-card" }),
    ],
});

beforeEach(() => {
    vi.clearAllMocks();
    showBoard({ data: discussed });
});

describe("BoardSummaryPage", () => {
    it("heads the summary with the board's title, stage, creation date and participants, and links back to it", () => {
        // when
        renderPage();

        // then
        expect(useBoard).toHaveBeenCalledWith(BOARD_ID);
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
        expect(screen.getByText("Discuss")).toBeInTheDocument();
        expect(screen.getByText("Created 01/10/2026")).toBeInTheDocument();
        expect(screen.getByText("1 participant")).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to board" })).toHaveAttribute(
            "href",
            `/o/harbour/boards/${BOARD_ID}`,
        );
    });

    it("lists each column's cards by votes, with merged cards under their parent", () => {
        // when
        renderPage();

        // then
        const start = within(screen.getByRole("list", { name: "Cards in Start" }));
        const cards = start.getAllByRole("article");
        expect(cards[0]).toHaveAccessibleName("Reviews were slow");
        expect(within(cards[0]).getByText("5 votes")).toBeInTheDocument();
        const merged = within(within(cards[0]).getByRole("list", { name: "Merged cards" }));
        expect(merged.getByRole("article", { name: "Reviews waited on one person" })).toHaveTextContent("2 votes");
        expect(start.getByRole("article", { name: "Standups ran long" })).toHaveTextContent("1 vote");
        expect(within(cards[0]).getByRole("list", { name: "Comments" })).toHaveTextContent("Pair on them");
    });

    it("puts each action item under the card it was made from, with its assignee, due date and status", () => {
        // when
        renderPage();

        // then
        const slow = within(screen.getByRole("article", { name: "Reviews were slow" }));
        const actions = slow.getAllByRole("list", { name: "Action items" })[0];
        expect(actions).toHaveTextContent("Ask @Sam to rotate reviewers");
        expect(actions).toHaveTextContent("Sam");
        expect(actions).toHaveTextContent("Due 15 Oct 2026");
        expect(actions).toHaveTextContent("Open");
        const standups = within(screen.getByRole("article", { name: "Standups ran long" }));
        expect(standups.getByRole("list", { name: "Action items" })).toHaveTextContent("Timebox standups");
        expect(standups.getByRole("list", { name: "Action items" })).toHaveTextContent("Done");
    });

    it("lists actions with no card the viewer can see under other actions, and counts hidden cards without their text", () => {
        // when
        renderPage();

        // then
        const other = within(screen.getByRole("region", { name: "Other actions" }));
        expect(other.getByText("Book the retro room")).toBeInTheDocument();
        expect(other.getByText("From a hidden card")).toBeInTheDocument();
        expect(other.queryByText("Timebox standups")).not.toBeInTheDocument();
        const stop = within(screen.getByRole("region", { name: "Stop" }));
        expect(stop.getByText("2 cards from others, hidden for now")).toBeInTheDocument();
        expect(stop.queryByRole("article")).not.toBeInTheDocument();
    });

    it("says an ended board ended, and shows no date an anonymous board does not carry", () => {
        // given
        showBoard({
            data: makeBoardSnapshot({
                board: makeBoardView({ closed: true, stage: "end", anonymity: "anonymous", created_at: undefined }),
            }),
        });

        // when
        renderPage();

        // then
        expect(screen.getByText("Ended")).toBeInTheDocument();
        expect(screen.queryByText(/^Created/)).not.toBeInTheDocument();
    });

    it("offers the board's downloads to a viewer who may export it", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        expect(screen.getByRole("menuitem", { name: "JSON" })).toHaveAttribute(
            "href",
            `/api/v1/boards/${BOARD_ID}/export?format=json`,
        );
    });

    it("offers no export to a viewer without board.export", () => {
        // given
        showBoard({ data: { ...discussed, viewer: makeParticipantViewer({ capabilities: ["board.view"] }) } });

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Export" })).not.toBeInTheDocument();
    });

    it("says the summary is loading", () => {
        // given
        showBoard({ loading: true });

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading summary...")).toBeInTheDocument();
    });

    it("shows a board that cannot be loaded as it failed, with a retry", async () => {
        // given
        const query = showBoard({ error: makeApiError({ status: 404, message: "board not found" }) });
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("board not found");
        expect(query.refetch).toHaveBeenCalledOnce();
    });

    it("points to the settings when the organisation is deleted", () => {
        // when
        renderPage("2026-10-01T09:00:00Z");

        // then
        expect(screen.getByRole("link", { name: "Go to organisation settings" })).toBeInTheDocument();
        expect(useBoard).not.toHaveBeenCalled();
    });
});
