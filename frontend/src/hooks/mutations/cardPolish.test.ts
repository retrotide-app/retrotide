import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeBoardCommandResponse, makeBoardSnapshot } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useAddReaction, usePinCard, useRemoveReaction } from "./cardPolish";

const endpoints = vi.hoisted(() => ({
    addReaction: vi.fn(),
    removeReaction: vi.fn(),
    pinCard: vi.fn(),
}));

vi.mock("../../api/endpoints/cardPolish", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const anyId = expect.stringMatching(V4);

const BOARD = "b1";

const BOARD_STATE = queryKeys.boardLive.state(BOARD);

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
}

const commands: CommandCase[] = [
    {
        name: "useAddReaction puts the reaction on the card",
        use: useAddReaction,
        variables: { cardId: "c1", reaction: "heart" },
        endpoint: endpoints.addReaction,
        request: [BOARD, "c1", "heart", { op_id: anyId }],
    },
    {
        name: "useRemoveReaction takes the reaction off the card",
        use: useRemoveReaction,
        variables: { cardId: "c1", reaction: "rocket" },
        endpoint: endpoints.removeReaction,
        request: [BOARD, "c1", "rocket", { op_id: anyId }],
    },
    {
        name: "usePinCard sends whether the card is pinned",
        use: usePinCard,
        variables: { cardId: "c1", pinned: true },
        endpoint: endpoints.pinCard,
        request: [BOARD, "c1", { op_id: anyId, pinned: true }],
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

describe("card polish commands", () => {
    it.each(commands)(
        "$name, and leaves the board for its own frame",
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
            expect(endpoint.mock.calls[0]).toEqual(request);
            expect(qc.getQueryData(BOARD_STATE)).toBe(held);
            expect(invalidate).not.toHaveBeenCalled();
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

    it.each([403, 404, 409])("refetches the board after a %i", async status => {
        // given
        endpoints.addReaction.mockRejectedValue(
            makeApiError({ status, message: "only a card everyone can see can be reacted to or pinned" }),
        );
        const { result, qc } = renderCommand(useAddReaction);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ cardId: "c1", reaction: "heart" })).rejects.toThrow(
                "only a card everyone can see can be reacted to or pinned",
            );
        });

        // then
        expectInvalidated(qc, BOARD_STATE);
    });

    it("leaves the board alone when the reaction is unknown", async () => {
        // given
        endpoints.addReaction.mockRejectedValue(
            makeApiError({ status: 400, message: "that is not one of the board reactions", code: "unknown_reaction" }),
        );
        const { result, invalidate } = renderCommand(useAddReaction);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ cardId: "c1", reaction: "heart" })).rejects.toThrow(
                "that is not one of the board reactions",
            );
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});
