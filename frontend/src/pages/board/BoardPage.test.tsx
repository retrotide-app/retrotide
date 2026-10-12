import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { type BoardSyncStatus, SYNC_CONNECTING, SYNC_LIVE } from "../../domain/board/syncStatus";
import type { QueryState } from "../../hooks/queries/queryState";
import {
    BOARD_ID,
    makeActionItem,
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeBoardViewer,
    makeCard,
    makeMutationState,
    makeOrganisation,
    makeParticipantViewer,
    makePreviousAction,
    makeQueryState,
    makeSpotlight,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardPage } from "./BoardPage";

const { useBoardView } = vi.hoisted(() => ({ useBoardView: vi.fn() }));

const mutations = vi.hoisted(() => ({
    useJoinBoard: vi.fn(),
    useDescribeBoard: vi.fn(),
    useCreateCard: vi.fn(),
    useEditCard: vi.fn(),
    useRecolourCard: vi.fn(),
    useDeleteCard: vi.fn(),
    useMoveCard: vi.fn(),
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
    useChangeStage: vi.fn(),
    useCloseBoard: vi.fn(),
    useBoardTimerCommand: vi.fn(),
    useUpdateBoardSettings: vi.fn(),
    useSetParticipantRole: vi.fn(),
    useRemoveParticipant: vi.fn(),
}));

const actionItemMutations = vi.hoisted(() => ({
    useAddActionItem: vi.fn(),
    useEditActionItem: vi.fn(),
    useMarkActionItem: vi.fn(),
    useDeleteActionItem: vi.fn(),
    useReviewPreviousAction: vi.fn(),
}));

const facilitationMutations = vi.hoisted(() => ({
    useRevealColumn: vi.fn(),
    useHideColumn: vi.fn(),
    useShowColumn: vi.fn(),
    useResetColumnVotes: vi.fn(),
    useSetColumnVoteLimit: vi.fn(),
    usePresentCard: vi.fn(),
    useHighlightCard: vi.fn(),
    useFocusColumn: vi.fn(),
}));

const { useBoardMembership } = vi.hoisted(() => ({ useBoardMembership: vi.fn() }));

const { useBoardTimer } = vi.hoisted(() => ({ useBoardTimer: vi.fn() }));

const viewport = vi.hoisted(() => ({
    isNarrowViewport: vi.fn(),
    subscribeNarrowViewport: vi.fn(),
}));

vi.mock("./ShareDialog", () => ({
    ShareDialog: ({ boardId, onClose }: { boardId: string; onClose: () => void }) => (
        <div role="dialog" aria-label="Share this board">
            <p>{`sharing ${boardId}`}</p>
            <button type="button" onClick={onClose}>
                Done
            </button>
        </div>
    ),
}));

vi.mock("../../components/board/surveys/SurveysPanel", () => ({
    SurveysPanel: ({ snapshot, writable }: { snapshot: BoardSnapshot; writable: boolean }) => (
        <p>{`surveys of ${snapshot.board.title}, writable ${String(writable)}`}</p>
    ),
}));

vi.mock("../../hooks/board/useBoardView", () => ({ useBoardView }));
vi.mock("../../hooks/board/useBoardTimer", () => ({ useBoardTimer }));
vi.mock("../../hooks/mutations/boards", () => mutations);
vi.mock("../../hooks/mutations/actionItems", () => actionItemMutations);
vi.mock("../../hooks/mutations/facilitation", () => facilitationMutations);
vi.mock("../../hooks/board/useBoardMembership", () => ({ useBoardMembership }));
vi.mock("../../platform/viewport", () => viewport);

const harbour = makeOrganisation({ slug: "harbour" });

const SHOWN_CARD_ID = "00000000-0000-4000-8000-000000000a02";

const discussing = makeBoardView({ stage: "discuss", cards_hidden: false });

const shownCard = makeCard({
    id: SHOWN_CARD_ID,
    body: "Fewer meetings",
    hidden: false,
    is_mine: undefined,
    author: { participant_id: "00000000-0000-4000-8000-0000000000f2", display_name: "Alex" },
});

function presentingBoard(viewer = makeBoardViewer()): BoardSnapshot {
    return makeBoardSnapshot({
        board: discussing,
        viewer,
        cards: [shownCard],
        spotlight: makeSpotlight({ presenting_card_id: SHOWN_CARD_ID }),
    });
}

function showView(state: Partial<QueryState<BoardSnapshot>>, sync: BoardSyncStatus = SYNC_LIVE) {
    const view = { ...makeQueryState<BoardSnapshot>(state), sync };
    useBoardView.mockReturnValue(view);

    return view;
}

function showBoard(snapshot: BoardSnapshot, sync: BoardSyncStatus = SYNC_LIVE) {
    showView({ data: snapshot }, sync);
}

function renderPage(deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, deleted_at: deletedAt });

    return renderWithProviders(<BoardPage />, {
        org: { organisations: [current], current },
        route: `/o/harbour/boards/${BOARD_ID}`,
        path: "/o/:orgSlug/boards/:boardId",
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    mutations.useJoinBoard.mockReturnValue(makeMutationState({ isSuccess: true }));
    for (const hook of Object.values(actionItemMutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    for (const hook of Object.values(facilitationMutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    useBoardMembership.mockReturnValue({ people: [], member: true, failure: null, retry: vi.fn() });
    viewport.isNarrowViewport.mockReturnValue(false);
    viewport.subscribeNarrowViewport.mockReturnValue(vi.fn());
    useBoardTimer.mockReturnValue(null);
    showBoard(makeBoardSnapshot());
});

describe("BoardPage", () => {
    it("joins the board on opening it", () => {
        // given
        const join = makeMutationState();
        mutations.useJoinBoard.mockReturnValue(join);

        // when
        renderPage();

        // then
        expect(useBoardView).toHaveBeenCalledWith(BOARD_ID);
        expect(mutations.useJoinBoard).toHaveBeenCalledWith(BOARD_ID);
        expect(join.mutate).toHaveBeenCalled();
    });

    it("says the board is live once it is joined", () => {
        // when
        renderPage();

        // then
        expect(screen.getByText("Live")).toBeInTheDocument();
        expect(screen.queryByText("Connecting")).not.toBeInTheDocument();
    });

    it("says it is connecting while the board is not yet live", () => {
        // given
        showBoard(makeBoardSnapshot(), SYNC_CONNECTING);

        // when
        renderPage();

        // then
        expect(screen.getByText("Connecting")).toBeInTheDocument();
        expect(screen.queryByText("Live")).not.toBeInTheDocument();
    });

    it("says why the board stopped updating when access is revoked, and keeps showing what it had", () => {
        // given
        showBoard(makeBoardSnapshot(), { state: "refused", reason: "removed", revoked: true });

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("A facilitator removed you from this board");
        expect(screen.getByRole("region", { name: "Start" })).toBeInTheDocument();
        expect(screen.queryByText("Live")).not.toBeInTheDocument();
    });

    it("shows the board's title, a way back to its team, and its columns", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to team" })).toHaveAttribute(
            "href",
            "/o/harbour/teams/00000000-0000-4000-8000-0000000000d1",
        );
        expect(screen.getByRole("region", { name: "Start" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Stop" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Continue" })).toBeInTheDocument();
    });

    it("leads back to the boards from a board that belongs to no team", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ team_id: undefined }) }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("link", { name: "Back to boards" })).toHaveAttribute("href", "/o/harbour");
        expect(screen.queryByRole("link", { name: "Back to team" })).not.toBeInTheDocument();
    });

    it("offers the column and list layouts, starting with columns on a wide screen", () => {
        // when
        renderPage();

        // then
        const layouts = screen.getByRole("group", { name: "Board layout" });
        expect(within(layouts).getByRole("button", { name: "Columns" })).toHaveAttribute("aria-pressed", "true");
        expect(within(layouts).getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "false");
        expect(screen.getAllByRole("list", { name: /^Cards in/ }).length).toBeGreaterThan(0);
    });

    it("starts a narrow screen on the list layout", () => {
        // given
        viewport.isNarrowViewport.mockReturnValue(true);

        // when
        renderPage();

        // then
        expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
        expect(screen.getByRole("region", { name: "Start" })).toBeInTheDocument();
    });

    it("shows no write controls until the join has gone through", () => {
        // given
        mutations.useJoinBoard.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("region", { name: "Start" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Board settings" })).not.toBeInTheDocument();
    });

    it("gives a facilitator who has joined a way to add cards and columns and a link to the board's settings", () => {
        // when
        renderPage();

        // then
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
        expect(screen.getByRole("link", { name: "Board settings" })).toHaveAttribute(
            "href",
            `/o/harbour/boards/${BOARD_ID}/settings`,
        );
        expect(screen.getByRole("button", { name: "Add a column" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Edit description" })).not.toBeInTheDocument();
    });

    it("lets a participant add cards but not reach the board's settings", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                board: makeBoardView({ context: "Release retro" }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByText("Release retro")).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Board settings" })).not.toBeInTheDocument();
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
    });

    it("lets a viewer who manages the board open the share dialog, and close it again", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Share" }));

        // then
        expect(screen.getByRole("dialog", { name: "Share this board" })).toHaveTextContent(`sharing ${BOARD_ID}`);

        // when
        await user.click(screen.getByRole("button", { name: "Done" }));

        // then
        expect(screen.queryByRole("dialog", { name: "Share this board" })).not.toBeInTheDocument();
    });

    it("never offers sharing to a participant, because the link is for managers only", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("button", { name: "Share" })).not.toBeInTheDocument();
    });

    it("offers no sharing before the join has gone through", () => {
        // given
        mutations.useJoinBoard.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("button", { name: "Share" })).not.toBeInTheDocument();
    });

    it("links everyone to the board's summary", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer({ capabilities: ["board.view"] }) }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("link", { name: "Summary" })).toHaveAttribute(
            "href",
            `/o/harbour/boards/${BOARD_ID}/summary`,
        );
    });

    it("offers the board's downloads to a viewer who may export it", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        expect(screen.getByRole("menuitem", { name: "CSV" })).toHaveAttribute(
            "href",
            `/api/v1/boards/${BOARD_ID}/export?format=csv`,
        );
        expect(screen.getByRole("menuitem", { name: "Markdown" })).toBeInTheDocument();
        expect(screen.getByRole("menuitem", { name: "JSON" })).toBeInTheDocument();
    });

    it("offers no export to a viewer without board.export", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer({ capabilities: ["board.view"] }) }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("button", { name: "Export" })).not.toBeInTheDocument();
    });

    it("shows the Prime Directive only when the board asks for it", () => {
        // when
        const { unmount } = renderPage();
        const shown = screen.queryByRole("note", { name: "The Prime Directive" });
        unmount();
        showBoard(makeBoardSnapshot({ board: makeBoardView({ show_prime_directive: false }) }));
        renderPage();

        // then
        expect(shown).not.toBeNull();
        expect(screen.queryByRole("note", { name: "The Prime Directive" })).not.toBeInTheDocument();
    });

    it("explains hidden cards, and draws other people's as placeholders with no text", () => {
        // when
        renderPage();

        // then
        expect(screen.getByText(/Cards are hidden/)).toBeInTheDocument();
        expect(screen.getByText("2 cards from others, hidden for now")).toBeInTheDocument();
        expect(screen.getByRole("article", { name: "Pair on reviews" })).toHaveTextContent("Only you can see this");
    });

    it("says an anonymous board hides authors", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ anonymity: "anonymous", cards_hidden: false }) }));

        // when
        renderPage();

        // then
        expect(screen.getByText(/This board is anonymous/)).toBeInTheDocument();
        expect(screen.queryByText(/Cards are hidden/)).not.toBeInTheDocument();
    });

    it("shows an archived board read only, without treating the refused join as a failure", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }) }), {
            state: "refused",
            reason: "archived",
            revoked: false,
        });
        mutations.useJoinBoard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "the board is archived", code: "archived" }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByText("Archived")).toBeInTheDocument();
        expect(screen.getByText("This board is archived, so it can only be read.")).toBeInTheDocument();
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
        expect(screen.queryByText("Live")).not.toBeInTheDocument();
        expect(screen.queryByText("Connecting")).not.toBeInTheDocument();
    });

    it("shows any other refused join and lets the viewer try again", async () => {
        // given
        const join = makeMutationState({
            isError: true,
            error: makeApiError({ status: 403, message: "you were removed from this board", code: "removed" }),
        });
        mutations.useJoinBoard.mockReturnValue(join);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("you were removed from this board");
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
        expect(join.mutate).toHaveBeenCalledTimes(2);
    });

    it("says the board is loading", () => {
        // given
        showView({ loading: true });

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading board...")).toBeInTheDocument();
    });

    it("shows a board that cannot be loaded as it failed, with a retry", async () => {
        // given
        const board = showView({ error: makeApiError({ status: 404, message: "board not found" }) });
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("board not found");
        expect(board.refetch).toHaveBeenCalledOnce();
    });

    it("gives a facilitator the facilitation bar on the live board", () => {
        // when
        renderPage();

        // then
        const bar = screen.getByRole("region", { name: "Facilitation" });
        expect(within(bar).getByRole("button", { name: "Next stage: Vote" })).toBeInTheDocument();
        expect(within(bar).getByRole("button", { name: "Start timer" })).toBeInTheDocument();
        expect(within(bar).getByRole("button", { name: "End retro" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Reveal cards" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Hide cards" })).not.toBeInTheDocument();
    });

    it("gives a participant no facilitation controls", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("region", { name: "Facilitation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "End retro" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: /Next stage/ })).not.toBeInTheDocument();
    });

    it("shows everyone the stage bar with the board's current stage", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                board: makeBoardView({ stage: "discuss", cards_hidden: false }),
                viewer: makeParticipantViewer(),
                hidden_counts: {},
            }),
        );

        // when
        renderPage();

        // then
        const stages = within(screen.getByRole("region", { name: "Retro stage" }));
        const current = stages.getAllByRole("listitem").find(step => step.getAttribute("aria-current") === "step");
        expect(current).toHaveTextContent("Discuss");
        expect(stages.getByText(/Vote totals are shown/)).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
    });

    it("shows the action items and a card's + Action in the Actions stage, with a way back to Discuss", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                board: makeBoardView({ stage: "actions", cards_hidden: false }),
                hidden_counts: {},
                cards: [makeCard({ id: "k1", body: "Standups ran long", hidden: false })],
                action_items: [makeActionItem({ title: "Book the room", source_card_id: "k1" })],
            }),
        );

        // when
        renderPage();

        // then
        const panel = within(screen.getByRole("region", { name: "Action items 1" }));
        const card = within(screen.getByRole("article", { name: "Standups ran long" }));
        expect(panel.getByText("Book the room")).toBeInTheDocument();
        expect(panel.queryByRole("form")).not.toBeInTheDocument();
        expect(card.getByRole("button", { name: "+ Action" })).toBeInTheDocument();
        expect(card.getByText("1 action")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Back to Discuss" })).toBeInTheDocument();
    });

    it("takes the viewer from an action item to the card it is for", async () => {
        // given
        const user = userEvent.setup();
        showBoard(
            makeBoardSnapshot({
                board: makeBoardView({ stage: "actions", cards_hidden: false }),
                hidden_counts: {},
                cards: [makeCard({ id: "k1", body: "Standups ran long", hidden: false })],
                action_items: [makeActionItem({ title: "Book the room", source_card_id: "k1" })],
            }),
        );
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "From: Standups ran long" }));

        // then
        expect(screen.getByRole("article", { name: "Standups ran long" })).toHaveFocus();
    });

    it("leaves the action items out before the Actions stage while there are none", () => {
        // when
        renderPage();

        // then
        expect(screen.queryByRole("region", { name: /Action items/ })).not.toBeInTheDocument();
    });

    it("reviews the team's previous actions on a new team board", () => {
        // given
        showBoard(makeBoardSnapshot({ previous_actions: [makePreviousAction({ title: "Write the release notes" })] }));

        // when
        renderPage();

        // then
        const panel = within(screen.getByRole("region", { name: "Previous actions 1" }));
        expect(panel.getByText("Write the release notes")).toBeInTheDocument();
        expect(panel.getByRole("button", { name: "Carry over" })).toBeInTheDocument();
    });

    it("ends the retro only once the facilitator confirms", async () => {
        // given
        const close = makeMutationState();
        mutations.useCloseBoard.mockReturnValue(close);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "End retro" }));
        const dialog = screen.getByRole("dialog", { name: "End this retro?" });
        await user.click(within(dialog).getByRole("button", { name: "End retro" }));

        // then
        expect(close.mutate).toHaveBeenCalledTimes(1);
    });

    it("shows an ended retro read only, with no facilitation bar", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ closed: true, cards_hidden: false, stage: "end" }) }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Ended")).toBeInTheDocument();
        expect(screen.getByText("The retro has ended, so the board can only be read.")).toBeInTheDocument();
        expect(screen.queryByText("Live")).not.toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Facilitation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
    });

    it("tells a participant which locks are on", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                settings: makeBoardSettings({ cards_locked: true }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("status", { name: "Locks" })).toHaveTextContent(
            "Cards are locked, so participants cannot add, edit or delete cards.",
        );
        expect(screen.queryByRole("button", { name: "Add a card" })).not.toBeInTheDocument();
    });

    it("points to the settings when the organisation is deleted", () => {
        // when
        renderPage("2026-10-01T09:00:00Z");

        // then
        expect(screen.getByRole("link", { name: "Go to organisation settings" })).toBeInTheDocument();
        expect(useBoardView).not.toHaveBeenCalled();
    });

    it("offers a facilitator the spotlight controls, with Present waiting until some cards are shown", () => {
        // when
        renderPage();

        // then
        const controls = screen.getByRole("region", { name: "Spotlight" });
        expect(within(controls).getByRole("button", { name: "Present" })).toBeDisabled();
        expect(within(controls).getByText("Reveal some cards first")).toBeInTheDocument();
        expect(within(controls).getByRole("combobox", { name: "Focus column" })).toBeInTheDocument();
    });

    it("never offers the spotlight controls to a participant", () => {
        // given
        showBoard(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // when
        renderPage();

        // then
        expect(screen.queryByRole("region", { name: "Spotlight" })).not.toBeInTheDocument();
    });

    it("starts a presentation at the first card everyone can see", async () => {
        // given
        const user = userEvent.setup();
        const present = makeMutationState();
        facilitationMutations.usePresentCard.mockReturnValue(present);
        showBoard(makeBoardSnapshot({ board: discussing, cards: [shownCard] }));
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Present" }));

        // then
        expect(present.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: SHOWN_CARD_ID });
    });

    it("replaces the columns with the presented card while a presentation runs", async () => {
        // given
        const user = userEvent.setup();
        const present = makeMutationState();
        facilitationMutations.usePresentCard.mockReturnValue(present);
        showBoard(presentingBoard());
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "End presentation" }));

        // then
        const presentation = screen.getByRole("region", { name: "Presentation" });
        expect(within(presentation).getByText("Card 1 of 1")).toBeInTheDocument();
        expect(within(presentation).getByRole("article", { name: "Presented card" })).toHaveTextContent(
            "Fewer meetings",
        );
        expect(within(presentation).getByRole("button", { name: "Next" })).toBeDisabled();
        expect(screen.queryByRole("heading", { name: "Start" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Present" })).not.toBeInTheDocument();
        expect(present.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: null });
    });

    it("lets a follower leave the presentation for the board and follow it again", async () => {
        // given
        const user = userEvent.setup();
        showBoard(presentingBoard(makeParticipantViewer()));
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Leave presentation" }));

        // then
        expect(screen.getByRole("status", { name: "Presentation" })).toHaveTextContent("The facilitator is presenting");
        expect(screen.getByRole("heading", { name: "Start" })).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Presentation" })).not.toBeInTheDocument();

        // when
        await user.click(screen.getByRole("button", { name: "Follow" }));

        // then
        expect(screen.getByRole("region", { name: "Presentation" })).toBeInTheDocument();
    });

    it("brings the highlighted card into view and tags it for everyone", () => {
        // given
        showBoard(
            makeBoardSnapshot({
                board: discussing,
                viewer: makeParticipantViewer(),
                cards: [shownCard],
                spotlight: makeSpotlight({ highlighted_card_id: SHOWN_CARD_ID }),
            }),
        );

        // when
        renderPage();

        // then
        const card = screen.getByRole("article", { name: "Fewer meetings" });
        expect(card).toHaveFocus();
        expect(within(card).getByText("Highlighted")).toBeInTheDocument();
    });

    it("hands the surveys panel the board and whether the viewer has joined", () => {
        // given
        mutations.useJoinBoard.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("surveys of Sprint 12, writable false")).toBeInTheDocument();
    });

    it("opens the keyboard shortcuts from the status area and with ?", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Keyboard shortcuts" }));
        const fromButton = screen.getByRole("dialog", { name: "Keyboard shortcuts" });
        await user.keyboard("{Escape}");
        const closed = screen.queryByRole("dialog", { name: "Keyboard shortcuts" });
        await user.keyboard("?");

        // then
        expect(fromButton).toHaveTextContent("Search the cards");
        expect(closed).not.toBeInTheDocument();
        expect(screen.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeInTheDocument();
    });

    it("focuses the card search with / and switches the layout with l", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.keyboard("/");
        const search = screen.getByRole("searchbox", { name: "Search cards" });
        const searchFocused = document.activeElement === search;
        search.blur();
        await user.keyboard("l");

        // then
        expect(searchFocused).toBe(true);
        expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
    });

    it("opens the first column's add-card form with n", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.keyboard("n");

        // then
        expect(screen.getByRole("form", { name: "Add a card to Start" })).toBeInTheDocument();
    });

    it("leaves the keyboard to the presentation while a card is presented", async () => {
        // given
        const user = userEvent.setup();
        showBoard(presentingBoard());
        renderPage();

        // when
        await user.keyboard("?");

        // then
        expect(screen.queryByRole("dialog", { name: "Keyboard shortcuts" })).not.toBeInTheDocument();
    });

    it("draws the board's background behind the cards", () => {
        // given
        showBoard(makeBoardSnapshot({ board: makeBoardView({ background_path: "/uploads/b1/m1.webp" }) }));

        // when
        renderPage();

        // then
        const canvas = screen.getByRole("toolbar", { name: "Cards" }).closest("[style]");
        expect(canvas).toHaveStyle({ backgroundImage: 'url("/uploads/b1/m1.webp")' });
    });

    it("draws no background when the board has none", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("toolbar", { name: "Cards" }).closest("[style]")).toBeNull();
    });

    it("hides a deleted card at once and brings it back on Undo, sending nothing", async () => {
        // given
        const user = userEvent.setup();
        const removal = makeMutationState();
        mutations.useDeleteCard.mockReturnValue(removal);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Card options" }));
        await user.click(screen.getByRole("menuitem", { name: "Delete card" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete this card?" })).getByRole("button", {
                name: "Delete card",
            }),
        );
        const hidden = screen.queryByRole("article", { name: "Pair on reviews" });
        const notice = screen.getByText("Card deleted.");
        await user.click(within(notice).getByRole("button", { name: "Undo" }));

        // then
        expect(hidden).not.toBeInTheDocument();
        expect(screen.getByRole("article", { name: "Pair on reviews" })).toBeInTheDocument();
        expect(screen.queryByText("Card deleted.")).not.toBeInTheDocument();
        expect(removal.mutate).not.toHaveBeenCalled();
        expect(removal.mutateAsync).not.toHaveBeenCalled();
    });

    it("sends a waiting delete at once when the viewer leaves the board", async () => {
        // given
        const user = userEvent.setup();
        const removal = makeMutationState();
        mutations.useDeleteCard.mockReturnValue(removal);
        const { unmount } = renderPage();
        await user.click(screen.getByRole("button", { name: "Card options" }));
        await user.click(screen.getByRole("menuitem", { name: "Delete card" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete this card?" })).getByRole("button", {
                name: "Delete card",
            }),
        );

        // when
        unmount();

        // then
        expect(removal.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "00000000-0000-4000-8000-000000000a01" });
    });
});
