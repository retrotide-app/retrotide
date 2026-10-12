import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeBoardCommandResponse, makeBoardSnapshot } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useFocusColumn,
    useHideColumn,
    useHighlightCard,
    usePresentCard,
    useResetColumnVotes,
    useRevealColumn,
    useSetColumnVoteLimit,
    useShowColumn,
} from "./facilitation";

const endpoints = vi.hoisted(() => ({
    focusColumn: vi.fn(),
    hideColumn: vi.fn(),
    highlightCard: vi.fn(),
    presentCard: vi.fn(),
    resetColumnVotes: vi.fn(),
    revealColumn: vi.fn(),
    setColumnVoteLimit: vi.fn(),
    showColumn: vi.fn(),
}));

vi.mock("../../api/endpoints/boards", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const BOARD = "b1";

const BOARD_STATE = queryKeys.boardLive.state(BOARD);

const anyId = expect.stringMatching(V4);

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
}

const commands: CommandCase[] = [
    {
        name: "useRevealColumn reveals one column's cards",
        use: useRevealColumn,
        variables: { columnId: "c1" },
        endpoint: endpoints.revealColumn,
        request: [BOARD, "c1", { op_id: anyId }],
    },
    {
        name: "useHideColumn hides a column from participants",
        use: useHideColumn,
        variables: { columnId: "c1" },
        endpoint: endpoints.hideColumn,
        request: [BOARD, "c1", { op_id: anyId }],
    },
    {
        name: "useShowColumn shows a hidden column to everyone",
        use: useShowColumn,
        variables: { columnId: "c1" },
        endpoint: endpoints.showColumn,
        request: [BOARD, "c1", { op_id: anyId }],
    },
    {
        name: "useResetColumnVotes resets one column's votes",
        use: useResetColumnVotes,
        variables: { columnId: "c1" },
        endpoint: endpoints.resetColumnVotes,
        request: [BOARD, "c1", { op_id: anyId }],
    },
    {
        name: "useSetColumnVoteLimit puts the column's vote limit",
        use: useSetColumnVoteLimit,
        variables: { columnId: "c1", maxVotes: 3 },
        endpoint: endpoints.setColumnVoteLimit,
        request: [BOARD, "c1", { op_id: anyId, max_votes: 3 }],
    },
    {
        name: "useSetColumnVoteLimit clears the column's vote limit",
        use: useSetColumnVoteLimit,
        variables: { columnId: "c1", maxVotes: null },
        endpoint: endpoints.setColumnVoteLimit,
        request: [BOARD, "c1", { op_id: anyId, max_votes: null }],
    },
    {
        name: "usePresentCard presents a card",
        use: usePresentCard,
        variables: { cardId: "k1" },
        endpoint: endpoints.presentCard,
        request: [BOARD, { op_id: anyId, card_id: "k1" }],
    },
    {
        name: "usePresentCard ends the presentation",
        use: usePresentCard,
        variables: { cardId: null },
        endpoint: endpoints.presentCard,
        request: [BOARD, { op_id: anyId, card_id: null }],
    },
    {
        name: "useHighlightCard highlights a card",
        use: useHighlightCard,
        variables: { cardId: "k1" },
        endpoint: endpoints.highlightCard,
        request: [BOARD, { op_id: anyId, card_id: "k1" }],
    },
    {
        name: "useFocusColumn focuses a column",
        use: useFocusColumn,
        variables: { columnId: "c1" },
        endpoint: endpoints.focusColumn,
        request: [BOARD, { op_id: anyId, column_id: "c1" }],
    },
    {
        name: "useFocusColumn ends the focus",
        use: useFocusColumn,
        variables: { columnId: null },
        endpoint: endpoints.focusColumn,
        request: [BOARD, { op_id: anyId, column_id: null }],
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

describe("facilitation commands", () => {
    it.each(commands)(
        "$name, and leaves the board for its own frame to update",
        async ({ use, variables, endpoint, request }) => {
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
            expect(invalidate).not.toHaveBeenCalled();
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

    it("refetches the board after a refusal such as a hidden column, because what is on show may be out of date", async () => {
        // given
        endpoints.presentCard.mockRejectedValue(
            makeApiError({ status: 409, code: "card_not_shown", message: "only a card everyone can see" }),
        );
        const { result, qc } = renderCommand(usePresentCard);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ cardId: "k1" })).rejects.toThrow("only a card everyone can see");
        });

        // then
        expectInvalidated(qc, BOARD_STATE);
    });
});
