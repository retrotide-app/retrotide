import type { QueryKey } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeBoardCommandResponse, makeBoardSnapshot, makeCard } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import type { BoardCommandResponse } from "../../types/api";
import {
    useBoardTimerCommand,
    useCastVote,
    useChangeStage,
    useClearColumn,
    useCloseBoard,
    useCreateBoard,
    useCreateCard,
    useCreateColumn,
    useCreateComment,
    useDeleteCard,
    useDeleteColumn,
    useDeleteComment,
    useDescribeBoard,
    useEditCard,
    useEditComment,
    useHideCards,
    useJoinBoard,
    useMergeCard,
    useMoveCard,
    useMoveColumn,
    usePendingBoardCommands,
    useRecolourCard,
    useRemoveParticipant,
    useRemoveVote,
    useResetVotes,
    useRevealCards,
    useSetParticipantRole,
    useUnmergeCard,
    useUpdateBoardSettings,
    useUpdateColumn,
} from "./boards";

const endpoints = vi.hoisted(() => ({
    castVote: vi.fn(),
    clearColumn: vi.fn(),
    createBoard: vi.fn(),
    createCard: vi.fn(),
    createColumn: vi.fn(),
    createComment: vi.fn(),
    deleteCard: vi.fn(),
    deleteColumn: vi.fn(),
    deleteComment: vi.fn(),
    describeBoard: vi.fn(),
    editCard: vi.fn(),
    editComment: vi.fn(),
    getBoard: vi.fn(),
    joinBoard: vi.fn(),
    listTeamBoards: vi.fn(),
    mergeCard: vi.fn(),
    moveCard: vi.fn(),
    moveColumn: vi.fn(),
    recolourCard: vi.fn(),
    removeVote: vi.fn(),
    resetVotes: vi.fn(),
    unmergeCard: vi.fn(),
    updateBoardSettings: vi.fn(),
    updateColumn: vi.fn(),
    revealCards: vi.fn(),
    hideCards: vi.fn(),
    changeStage: vi.fn(),
    runTimer: vi.fn(),
    closeBoard: vi.fn(),
    setParticipantRole: vi.fn(),
    removeParticipant: vi.fn(),
}));

vi.mock("../../api/endpoints/boards", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const BOARD = "b1";

const BOARD_STATE = queryKeys.boardLive.state(BOARD);

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
    refreshes?: QueryKey[];
}

const anyId = expect.stringMatching(V4);

const MENTIONS = queryKeys.auth.mentions();

const commands: CommandCase[] = [
    {
        name: "useDescribeBoard puts the board's description",
        use: useDescribeBoard,
        variables: { context: "Release week" },
        endpoint: endpoints.describeBoard,
        request: [BOARD, { context: "Release week", op_id: anyId }],
    },
    {
        name: "useCreateColumn adds a column with a new v4 id",
        use: useCreateColumn,
        variables: { title: "Kudos", colour: "teal" },
        endpoint: endpoints.createColumn,
        request: [BOARD, { title: "Kudos", colour: "teal", op_id: anyId, id: anyId }],
    },
    {
        name: "useUpdateColumn puts the column's fields",
        use: useUpdateColumn,
        variables: { columnId: "c1", title: "Kudos", description: "Thanks", colour: "pink" },
        endpoint: endpoints.updateColumn,
        request: [BOARD, "c1", { title: "Kudos", description: "Thanks", colour: "pink", op_id: anyId }],
    },
    {
        name: "useMoveColumn sends the column it should follow",
        use: useMoveColumn,
        variables: { columnId: "c1", after_column_id: null },
        endpoint: endpoints.moveColumn,
        request: [BOARD, "c1", { after_column_id: null, op_id: anyId }],
    },
    {
        name: "useClearColumn clears the column",
        use: useClearColumn,
        variables: { columnId: "c1" },
        endpoint: endpoints.clearColumn,
        request: [BOARD, "c1", { op_id: anyId }],
    },
    {
        name: "useDeleteColumn deletes the column",
        use: useDeleteColumn,
        variables: { columnId: "c1" },
        endpoint: endpoints.deleteColumn,
        request: [BOARD, "c1", { op_id: anyId }],
    },
    {
        name: "useCreateCard adds a card with a new v4 id",
        use: useCreateCard,
        variables: { column_id: "c1", body: "Ship it", colour: null },
        endpoint: endpoints.createCard,
        request: [BOARD, { column_id: "c1", body: "Ship it", colour: null, op_id: anyId, id: anyId }],
        refreshes: [MENTIONS],
    },
    {
        name: "useCreateCard sends the people the card mentions",
        use: useCreateCard,
        variables: { column_id: "c1", body: "Ask @Ali", colour: null, mentions: ["u2"] },
        endpoint: endpoints.createCard,
        request: [
            BOARD,
            { column_id: "c1", body: "Ask @Ali", colour: null, mentions: ["u2"], op_id: anyId, id: anyId },
        ],
        refreshes: [MENTIONS],
    },
    {
        name: "useEditCard puts the card's new text with everyone it still mentions",
        use: useEditCard,
        variables: { cardId: "k1", body: "Ship it today @Ali", mentions: ["u2"] },
        endpoint: endpoints.editCard,
        request: [BOARD, "k1", { body: "Ship it today @Ali", mentions: ["u2"], op_id: anyId }],
        refreshes: [MENTIONS],
    },
    {
        name: "useRecolourCard puts the card's colour",
        use: useRecolourCard,
        variables: { cardId: "k1", colour: "amber" },
        endpoint: endpoints.recolourCard,
        request: [BOARD, "k1", { colour: "amber", op_id: anyId }],
    },
    {
        name: "useMoveCard sends the column and the card it should follow",
        use: useMoveCard,
        variables: { cardId: "k1", column_id: "c2", after_card_id: "k2" },
        endpoint: endpoints.moveCard,
        request: [BOARD, "k1", { column_id: "c2", after_card_id: "k2", op_id: anyId }],
    },
    {
        name: "useDeleteCard deletes the card",
        use: useDeleteCard,
        variables: { cardId: "k1" },
        endpoint: endpoints.deleteCard,
        request: [BOARD, "k1", { op_id: anyId }],
        refreshes: [MENTIONS],
    },
    {
        name: "useCastVote votes for the card",
        use: useCastVote,
        variables: { cardId: "k1" },
        endpoint: endpoints.castVote,
        request: [BOARD, "k1", { op_id: anyId }],
    },
    {
        name: "useRemoveVote takes a vote back from the card",
        use: useRemoveVote,
        variables: { cardId: "k1" },
        endpoint: endpoints.removeVote,
        request: [BOARD, "k1", { op_id: anyId }],
    },
    {
        name: "useResetVotes resets every vote on the board",
        use: useResetVotes,
        variables: undefined,
        endpoint: endpoints.resetVotes,
        request: [BOARD, { op_id: anyId }],
    },
    {
        name: "useUpdateBoardSettings puts only the settings that changed",
        use: useUpdateBoardSettings,
        variables: { vote_counts_hidden: true },
        endpoint: endpoints.updateBoardSettings,
        request: [BOARD, { vote_counts_hidden: true, op_id: anyId }],
    },
    {
        name: "useMergeCard merges the card into its target",
        use: useMergeCard,
        variables: { cardId: "k1", targetCardId: "k2" },
        endpoint: endpoints.mergeCard,
        request: [BOARD, "k1", { op_id: anyId, target_card_id: "k2" }],
    },
    {
        name: "useUnmergeCard takes the card out of its stack",
        use: useUnmergeCard,
        variables: { cardId: "k2" },
        endpoint: endpoints.unmergeCard,
        request: [BOARD, "k2", { op_id: anyId }],
    },
    {
        name: "useCreateComment adds a comment with a new v4 id and the people it mentions",
        use: useCreateComment,
        variables: { cardId: "k1", body: "Agreed @Ali", mentions: ["u2"] },
        endpoint: endpoints.createComment,
        request: [BOARD, "k1", { body: "Agreed @Ali", mentions: ["u2"], op_id: anyId, id: anyId }],
        refreshes: [MENTIONS],
    },
    {
        name: "useEditComment puts the comment's new text with everyone it still mentions",
        use: useEditComment,
        variables: { commentId: "m1", body: "Agreed, mostly", mentions: [] },
        endpoint: endpoints.editComment,
        request: [BOARD, "m1", { body: "Agreed, mostly", mentions: [], op_id: anyId }],
        refreshes: [MENTIONS],
    },
    {
        name: "useDeleteComment deletes the comment",
        use: useDeleteComment,
        variables: { commentId: "m1" },
        endpoint: endpoints.deleteComment,
        request: [BOARD, "m1", { op_id: anyId }],
        refreshes: [MENTIONS],
    },
    {
        name: "useRevealCards reveals every card",
        use: useRevealCards,
        variables: undefined,
        endpoint: endpoints.revealCards,
        request: [BOARD, { op_id: anyId }],
    },
    {
        name: "useHideCards hides the cards again",
        use: useHideCards,
        variables: undefined,
        endpoint: endpoints.hideCards,
        request: [BOARD, { op_id: anyId }],
    },
    {
        name: "useChangeStage names the stage the board moves on to",
        use: useChangeStage,
        variables: { stage: "discuss" },
        endpoint: endpoints.changeStage,
        request: [BOARD, { op_id: anyId, stage: "discuss" }],
    },
    {
        name: "useBoardTimerCommand starts the timer with its length",
        use: useBoardTimerCommand,
        variables: { action: "start", duration_ms: 300000 },
        endpoint: endpoints.runTimer,
        request: [BOARD, { action: "start", duration_ms: 300000, op_id: anyId }],
    },
    {
        name: "useBoardTimerCommand pauses the timer",
        use: useBoardTimerCommand,
        variables: { action: "pause" },
        endpoint: endpoints.runTimer,
        request: [BOARD, { action: "pause", op_id: anyId }],
    },
    {
        name: "useCloseBoard ends the retro",
        use: useCloseBoard,
        variables: undefined,
        endpoint: endpoints.closeBoard,
        request: [BOARD, { op_id: anyId }],
    },
    {
        name: "useSetParticipantRole makes a participant a co-facilitator",
        use: useSetParticipantRole,
        variables: { participantId: "p1", role: "facilitator" },
        endpoint: endpoints.setParticipantRole,
        request: [BOARD, "p1", { op_id: anyId, role: "facilitator" }],
    },
    {
        name: "useRemoveParticipant removes the participant from the board",
        use: useRemoveParticipant,
        variables: { participantId: "p1" },
        endpoint: endpoints.removeParticipant,
        request: [BOARD, "p1", { op_id: anyId }],
    },
];

function renderCommand<T>(use: (boardId: string) => T) {
    const qc = createTestQueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    const rendered = renderHook(() => use(BOARD), { wrapper: providerWrapper({ queryClient: qc }) });

    return { ...rendered, qc, invalidate };
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
        fn.mockResolvedValue(makeBoardCommandResponse());
    }
});

describe("board commands", () => {
    it.each(commands)(
        "$name, and leaves the board for its own frame to update rather than refetching it",
        async ({ use, variables, endpoint, request, refreshes = [] }) => {
            // given
            const { result, qc, invalidate } = renderCommand(use);
            const held = makeBoardSnapshot({ board_id: BOARD, version: 3 });
            qc.setQueryDefaults(BOARD_STATE, { gcTime: Infinity });
            qc.setQueryData(BOARD_STATE, held);

            // when
            await act(async () => {
                await result.current.mutateAsync(variables as never);
            });

            // then
            expect(endpoint).toHaveBeenCalledTimes(1);
            expect(endpoint.mock.calls[0]).toEqual(request);
            expect(invalidate.mock.calls).toEqual(refreshes.map(queryKey => [{ queryKey }]));
            expect(qc.getQueryData(BOARD_STATE)).toBe(held);
        },
    );

    it.each(commands)("files the command under the board's mutation key: $name", async ({ use, variables }) => {
        // given
        const { result, qc } = renderCommand(use);

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
        });

        // then
        const filed = qc.getMutationCache().findAll({ mutationKey: queryKeys.boardLive.mutations(BOARD) });
        expect(filed).toHaveLength(1);
    });

    it.each(commands)("sends a fresh op id with every command: $name", async ({ use, variables, endpoint }) => {
        // given
        const { result } = renderCommand(use);

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
            await result.current.mutateAsync(variables as never);
        });

        // then
        const opIds = endpoint.mock.calls.map(call => (call.at(-1) as { op_id: string }).op_id);
        expect(opIds).toHaveLength(2);
        expect(opIds[0]).not.toBe(opIds[1]);
    });

    it.each([403, 404, 409, 500])(
        "refetches the board after a %i, because what is on show may be out of date",
        async status => {
            // given
            endpoints.createCard.mockRejectedValue(makeApiError({ status, message: "changed meanwhile" }));
            const { result, qc, invalidate } = renderCommand(useCreateCard);

            // when
            await act(async () => {
                await expect(
                    result.current.mutateAsync({ column_id: "c1", body: "Ship it", colour: null }),
                ).rejects.toThrow("changed meanwhile");
            });

            // then
            expectInvalidated(qc, BOARD_STATE);
            expect(invalidate).toHaveBeenCalledTimes(1);
        },
    );

    it.each([400, 429, 503])("leaves the board alone after a %i, which changed nothing", async status => {
        // given
        endpoints.editCard.mockRejectedValue(makeApiError({ status, message: "refused", code: "card_body_required" }));
        const { result, invalidate } = renderCommand(useEditCard);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ cardId: "k1", body: "", mentions: [] })).rejects.toThrow(
                "refused",
            );
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("gives every new card its own id", async () => {
        // given
        const { result } = renderCommand(useCreateCard);

        // when
        await act(async () => {
            await result.current.mutateAsync({ column_id: "c1", body: "One", colour: null });
            await result.current.mutateAsync({ column_id: "c1", body: "Two", colour: null });
        });

        // then
        const ids = endpoints.createCard.mock.calls.map(call => (call[1] as { id: string; op_id: string }).id);
        expect(ids[0]).not.toBe(ids[1]);
    });

    it("never writes a move into the confirmed board, which only the server's frames change", async () => {
        // given
        const board = makeBoardSnapshot();
        const [first, second] = board.columns;
        const card = makeCard({ id: "k1", column_id: first.id, position: "a0" });
        const { result, qc } = renderCommand(useMoveCard);
        qc.setQueryDefaults(BOARD_STATE, { gcTime: Infinity });
        qc.setQueryData(BOARD_STATE, { ...board, cards: [card] });
        let seenWhileSending: string | undefined;
        endpoints.moveCard.mockImplementation(() => {
            seenWhileSending = qc.getQueryData<typeof board>(BOARD_STATE)?.cards[0].column_id;

            return Promise.resolve(makeBoardCommandResponse());
        });

        // when
        await act(async () => {
            await result.current.mutateAsync({ cardId: "k1", column_id: second.id, after_card_id: null });
        });

        // then
        expect(seenWhileSending).toBe(first.id);
        expect(qc.getQueryData<typeof board>(BOARD_STATE)?.cards[0]).toEqual(card);
    });

    it("hands back the server's answer, with the author's own card", async () => {
        // given
        const answer = makeBoardCommandResponse({ version: 9, card: makeCard({ id: "k7" }) });
        endpoints.createCard.mockResolvedValue(answer);
        const { result } = renderCommand(useCreateCard);

        // when
        let created: unknown;
        await act(async () => {
            created = await result.current.mutateAsync({ column_id: "c1", body: "Ship it" });
        });

        // then
        expect(created).toEqual(answer);
    });
});

describe("usePendingBoardCommands", () => {
    function renderPending() {
        return renderCommand(boardId => ({
            moveCard: useMoveCard(boardId),
            moveColumn: useMoveColumn(boardId),
            createCard: useCreateCard(boardId),
            pending: usePendingBoardCommands(boardId),
        }));
    }

    it("lists a card move while its request is out, with no answer yet", async () => {
        // given
        endpoints.moveCard.mockReturnValue(new Promise<BoardCommandResponse>(() => undefined));
        const { result } = renderPending();

        // when
        act(() => {
            result.current.moveCard.mutate({ cardId: "k1", column_id: "c2", after_card_id: "k2" });
        });

        // then
        await waitFor(() =>
            expect(result.current.pending).toEqual([
                {
                    change: { kind: "card-move", cardId: "k1", drop: { column_id: "c2", after_card_id: "k2" } },
                    answeredAt: null,
                },
            ]),
        );
    });

    it("keeps an answered move with the version the server answered, so it stays shown until its frame lands", async () => {
        // given
        endpoints.moveColumn.mockResolvedValue(makeBoardCommandResponse({ version: 12 }));
        const { result } = renderPending();

        // when
        await act(async () => {
            await result.current.moveColumn.mutateAsync({ columnId: "c1", after_column_id: null });
        });

        // then
        await waitFor(() =>
            expect(result.current.pending).toEqual([
                { change: { kind: "column-move", columnId: "c1", drop: { after_column_id: null } }, answeredAt: 12 },
            ]),
        );
    });

    it("drops a refused move, so its change disappears without a rollback", async () => {
        // given
        endpoints.moveCard.mockRejectedValue(makeApiError({ status: 400, message: "refused" }));
        const { result } = renderPending();

        // when
        await act(async () => {
            await result.current.moveCard
                .mutateAsync({ cardId: "k1", column_id: "c2", after_card_id: null })
                .catch(() => undefined);
        });

        // then
        await waitFor(() => expect(result.current.pending).toEqual([null]));
    });

    it("lists a vote and a taken-back vote while their requests are out", async () => {
        // given
        endpoints.castVote.mockReturnValue(new Promise<BoardCommandResponse>(() => undefined));
        endpoints.removeVote.mockReturnValue(new Promise<BoardCommandResponse>(() => undefined));
        const { result } = renderCommand(boardId => ({
            cast: useCastVote(boardId),
            remove: useRemoveVote(boardId),
            pending: usePendingBoardCommands(boardId),
        }));

        // when
        act(() => {
            result.current.cast.mutate({ cardId: "k1" });
            result.current.remove.mutate({ cardId: "k2" });
        });

        // then
        await waitFor(() =>
            expect(result.current.pending).toEqual([
                { change: { kind: "vote", cardId: "k1", delta: 1 }, answeredAt: null },
                { change: { kind: "vote", cardId: "k2", delta: -1 }, answeredAt: null },
            ]),
        );
    });

    it("lists commands it cannot show early as nothing", async () => {
        // given
        const { result } = renderPending();

        // when
        await act(async () => {
            await result.current.createCard.mutateAsync({ column_id: "c1", body: "Ship it", colour: null });
        });

        // then
        await waitFor(() => expect(result.current.pending).toEqual([null]));
    });
});

describe("the viewer's own votes in a vote's answer", () => {
    it("keeps the votes the answer carries, at the answer's version", async () => {
        // given
        endpoints.castVote.mockResolvedValue(
            makeBoardCommandResponse({ version: 5, me: { votes_remaining: 4, my_votes: { k1: 2 } } }),
        );
        const { result, qc } = renderCommand(useCastVote);
        qc.setQueryDefaults(BOARD_STATE, { gcTime: Infinity });
        qc.setQueryData(BOARD_STATE, makeBoardSnapshot({ board_id: BOARD, version: 3 }));

        // when
        await act(async () => {
            await result.current.mutateAsync({ cardId: "k1" });
        });

        // then
        const held = qc.getQueryData<ReturnType<typeof makeBoardSnapshot>>(BOARD_STATE);
        expect(held?.me).toEqual({ votes_remaining: 4, my_votes: { k1: 2 }, version: 5 });
        expect(held?.version).toBe(3);
    });

    it("keeps newer votes it already holds from a board_me frame", async () => {
        // given
        endpoints.removeVote.mockResolvedValue(
            makeBoardCommandResponse({ version: 5, me: { votes_remaining: 6, my_votes: {} } }),
        );
        const { result, qc } = renderCommand(useRemoveVote);
        const newer = makeBoardSnapshot({
            board_id: BOARD,
            version: 3,
            me: { votes_remaining: 5, my_votes: { k2: 1 }, version: 7 },
        });
        qc.setQueryDefaults(BOARD_STATE, { gcTime: Infinity });
        qc.setQueryData(BOARD_STATE, newer);

        // when
        await act(async () => {
            await result.current.mutateAsync({ cardId: "k1" });
        });

        // then
        expect(qc.getQueryData(BOARD_STATE)).toBe(newer);
    });
});

describe("useCreateBoard", () => {
    it("creates a team board, holds its snapshot for the board page and refreshes the team's and organisations' boards", async () => {
        // given
        const qc = createTestQueryClient();
        qc.setQueryDefaults(queryKeys.boardLive.state("b9"), { gcTime: Infinity });
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const snapshot = makeBoardSnapshot({ board_id: "b9" });
        endpoints.createBoard.mockResolvedValue(snapshot);
        const { result } = renderHook(useCreateBoard, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let created: unknown;
        await act(async () => {
            created = await result.current.mutateAsync({
                orgId: "o1",
                team_id: "t1",
                title: "Sprint 12",
                template_key: "blank",
            });
        });

        // then
        expect(endpoints.createBoard).toHaveBeenCalledWith("o1", {
            team_id: "t1",
            title: "Sprint 12",
            template_key: "blank",
        });
        expect(created).toEqual(snapshot);
        expect(qc.getQueryData(queryKeys.boardLive.state("b9"))).toEqual(snapshot);
        expectInvalidated(qc, queryKeys.boards.team("t1"));
        expectInvalidated(qc, queryKeys.boards.organisations);
        expect(invalidate).toHaveBeenCalledTimes(2);
    });

    it("creates an organisation board with no team and refreshes only the organisations' boards", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoints.createBoard.mockResolvedValue(makeBoardSnapshot({ board_id: "b9" }));
        const { result } = renderHook(useCreateBoard, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync({ orgId: "o1", title: "Sprint 12", template_key: "blank" });
        });

        // then
        expect(endpoints.createBoard).toHaveBeenCalledWith("o1", { title: "Sprint 12", template_key: "blank" });
        expectInvalidated(qc, queryKeys.boards.organisations);
        expect(invalidate).toHaveBeenCalledTimes(1);
    });

    it.each([404, 409])("refreshes the board lists after a %i, because the team changed", async status => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoints.createBoard.mockRejectedValue(
            makeApiError({ status, message: "the team is archived", code: "team_archived" }),
        );
        const { result } = renderHook(useCreateBoard, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({ orgId: "o1", team_id: "t1", title: "Sprint 12", template_key: "blank" }),
            ).rejects.toThrow("the team is archived");
        });

        // then
        expectInvalidated(qc, queryKeys.boards.team("t1"));
        expectInvalidated(qc, queryKeys.boards.organisations);
        expect(invalidate).toHaveBeenCalledTimes(2);
    });

    it("leaves every cache alone when the title is refused", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoints.createBoard.mockRejectedValue(
            makeApiError({ status: 400, message: "a board title is required", code: "board_title_required" }),
        );
        const { result } = renderHook(useCreateBoard, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ orgId: "o1", title: "", template_key: "blank" })).rejects.toThrow(
                "a board title is required",
            );
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useJoinBoard", () => {
    it("joins the board as the signed in member, then refetches the board", async () => {
        // given
        endpoints.joinBoard.mockResolvedValue(undefined);
        const { result, qc, invalidate } = renderCommand(useJoinBoard);

        // when
        await act(async () => {
            await result.current.mutateAsync(undefined as never);
        });

        // then
        expect(endpoints.joinBoard).toHaveBeenCalledWith(BOARD);
        expectInvalidated(qc, BOARD_STATE);
        expect(invalidate).toHaveBeenCalledTimes(1);
    });

    it("hands back the refusal of a removed participant", async () => {
        // given
        const removed = makeApiError({ status: 403, message: "you were removed from this board", code: "removed" });
        endpoints.joinBoard.mockRejectedValue(removed);
        const { result } = renderCommand(useJoinBoard);

        // when
        let thrown: unknown;
        await act(async () => {
            thrown = await result.current.mutateAsync(undefined as never).catch((error: unknown) => error);
        });

        // then
        expect(thrown).toBe(removed);
    });
});
