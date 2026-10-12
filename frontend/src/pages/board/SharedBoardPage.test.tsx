import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { rejoinState } from "../../domain/board/shareLink";
import { type BoardSyncStatus, SYNC_LIVE } from "../../domain/board/syncStatus";
import type { QueryState } from "../../hooks/queries/queryState";
import {
    BOARD_ID,
    makeApiError,
    makeBoardSnapshot,
    makeMutationState,
    makeParticipantViewer,
    makeQueryState,
    makeSessionUser,
    makeTimer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { SharedBoardPage } from "./SharedBoardPage";

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

const { useBoardTimer } = vi.hoisted(() => ({ useBoardTimer: vi.fn() }));

const viewport = vi.hoisted(() => ({
    isNarrowViewport: vi.fn(),
    subscribeNarrowViewport: vi.fn(),
}));

vi.mock("../../hooks/board/useBoardView", () => ({ useBoardView }));
vi.mock("../../hooks/board/useBoardTimer", () => ({ useBoardTimer }));
vi.mock("../../hooks/mutations/boards", () => mutations);
vi.mock("../../platform/viewport", () => viewport);
vi.mock("./ShareDialog", () => ({
    ShareDialog: () => <div role="dialog" aria-label="Share this board" />,
}));

const guestSnapshot = makeBoardSnapshot({
    viewer: makeParticipantViewer({ capabilities: ["board.view", "board.participate"] }),
});

function showView(state: Partial<QueryState<BoardSnapshot>>, sync: BoardSyncStatus = SYNC_LIVE) {
    const view = { ...makeQueryState<BoardSnapshot>(state), sync };
    useBoardView.mockReturnValue(view);

    return view;
}

interface PageOptions {
    guestId?: string | null;
    signedIn?: boolean;
    joinSlug?: string;
}

function renderPage({ guestId = "g1", signedIn = false, joinSlug }: PageOptions = {}) {
    return renderWithProviders(<SharedBoardPage />, {
        user: signedIn ? makeSessionUser() : null,
        auth: { guestId },
        route: `/boards/${BOARD_ID}`,
        routeState: joinSlug === undefined ? undefined : rejoinState(joinSlug),
        path: "/boards/:boardId",
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    mutations.useJoinBoard.mockReturnValue(makeMutationState({ isSuccess: true }));
    viewport.isNarrowViewport.mockReturnValue(false);
    viewport.subscribeNarrowViewport.mockReturnValue(vi.fn());
    useBoardTimer.mockReturnValue(null);
    showView({ data: guestSnapshot });
});

describe("SharedBoardPage", () => {
    it("opens the board the link joined, live, without any organisation around it", () => {
        // when
        renderPage({ joinSlug: "k7Q2" });

        // then
        expect(useBoardView).toHaveBeenCalledWith(BOARD_ID);
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
        expect(screen.getByText("Live")).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: /Back to/ })).not.toBeInTheDocument();
        expect(screen.getAllByRole("button", { name: "Add a card" })).toHaveLength(3);
    });

    it("never offers a guest sharing or the board's settings", () => {
        // when
        renderPage();

        // then
        expect(screen.queryByRole("button", { name: "Share" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Board settings" })).not.toBeInTheDocument();
    });

    it("links a guest to the board's summary outside any organisation, with no export they were not given", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("link", { name: "Summary" })).toHaveAttribute("href", `/boards/${BOARD_ID}/summary`);
        expect(screen.queryByRole("button", { name: "Export" })).not.toBeInTheDocument();
    });

    it("lets a manager who came through the link share the board, but not reach organisation settings", () => {
        // given
        showView({ data: makeBoardSnapshot() });

        // when
        renderPage({ guestId: null, signedIn: true });

        // then
        expect(screen.getByRole("button", { name: "Share" })).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Board settings" })).not.toBeInTheDocument();
    });

    it("tells a guest whose pass has gone to join again through the link, not that their session expired", () => {
        // given
        showView({ error: makeApiError({ status: 401, message: "invalid or expired session" }) });

        // when
        renderPage({ joinSlug: "k7Q2" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This browser no longer holds its guest pass for the board. Open the board's link again to rejoin.",
        );
        expect(screen.getByRole("link", { name: "Join again" })).toHaveAttribute("href", "/b/k7Q2");
        expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
    });

    it("tells a guest whose pass was swept to join again, even though the session no longer names the guest", () => {
        // given
        showView({ error: makeApiError({ status: 401, message: "invalid or expired session" }) });

        // when
        renderPage({ guestId: null, joinSlug: "k7Q2" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This browser no longer holds its guest pass for the board. Open the board's link again to rejoin.",
        );
        expect(screen.getByRole("link", { name: "Join again" })).toHaveAttribute("href", "/b/k7Q2");
    });

    it("offers no rejoin link when the page no longer knows which link was used", () => {
        // given
        showView({ error: makeApiError({ status: 401, message: "invalid or expired session" }) });

        // when
        renderPage();

        // then
        expect(screen.queryByRole("link", { name: "Join again" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("asks a visitor with no pass to sign in", () => {
        // given
        showView({ error: makeApiError({ status: 401, message: "authentication required" }) });

        // when
        renderPage({ guestId: null });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Sign in, or open the board's link, to see this board.");
        expect(screen.getByRole("link", { name: "Sign in" })).toBeInTheDocument();
    });

    it("asks for the new password when it changed while the guest was away", () => {
        // given
        showView({
            error: makeApiError({ status: 403, message: "the board needs its password", code: "password_required" }),
        });

        // when
        renderPage({ joinSlug: "k7Q2" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This board's password changed. Open the board's link again and enter the new one.",
        );
        expect(screen.getByRole("link", { name: "Join again" })).toHaveAttribute("href", "/b/k7Q2");
    });

    it("says why the board stopped updating when the guest is evicted live, and offers to join again", () => {
        // given
        showView({ data: guestSnapshot }, { state: "refused", reason: "password_required", revoked: true });

        // when
        renderPage({ joinSlug: "k7Q2" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This board needs its password again before it can update live.",
        );
        expect(screen.getByRole("link", { name: "Join again" })).toHaveAttribute("href", "/b/k7Q2");
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
    });

    it("shows a failure that is not a refusal as it failed, with a retry", async () => {
        // given
        const board = showView({ error: makeApiError({ status: 503, message: "service unavailable" }) });
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(board.refetch).toHaveBeenCalledOnce();
    });

    it("shows a guest the running timer, with no way to change it", () => {
        // given
        const timer = makeTimer();
        showView({ data: { ...guestSnapshot, timer } });
        useBoardTimer.mockReturnValue({ timer, reading: { remainingMs: 95_000, phase: "running", tone: "calm" } });

        // when
        renderPage();

        // then
        expect(screen.getByRole("timer", { name: "Time left" })).toHaveTextContent("1:35");
        expect(screen.queryByRole("button", { name: "Pause" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "End retro" })).not.toBeInTheDocument();
    });
});
