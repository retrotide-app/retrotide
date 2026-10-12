import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeActionItem, makeApiError, makeBoardCommandResponse, makeBoardSnapshot } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useAddActionItem,
    useDeleteActionItem,
    useEditActionItem,
    useMarkActionItem,
    useMarkListedActionItem,
    useReviewPreviousAction,
} from "./actionItems";

const endpoints = vi.hoisted(() => ({
    addActionItem: vi.fn(),
    editActionItem: vi.fn(),
    markActionItem: vi.fn(),
    deleteActionItem: vi.fn(),
    reviewPreviousAction: vi.fn(),
    listBoardPeople: vi.fn(),
    listOrganisationActionItems: vi.fn(),
    listMentions: vi.fn(),
}));

vi.mock("../../api/endpoints/actionItems", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const anyId = expect.stringMatching(V4);

const BOARD = "b1";

const BOARD_STATE = queryKeys.boardLive.state(BOARD);

const FIELDS = { title: "Ask @Ali", assignee_user_id: "u1", due_on: "2026-10-20", mentions: ["u2"] };

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
}

const commands: CommandCase[] = [
    {
        name: "useAddActionItem adds an item with a new v4 id",
        use: useAddActionItem,
        variables: { ...FIELDS, source_card_id: "k1" },
        endpoint: endpoints.addActionItem,
        request: [BOARD, { ...FIELDS, source_card_id: "k1", op_id: anyId, id: anyId }],
    },
    {
        name: "useEditActionItem puts every field of the item",
        use: useEditActionItem,
        variables: { itemId: "i1", ...FIELDS },
        endpoint: endpoints.editActionItem,
        request: [BOARD, "i1", { ...FIELDS, op_id: anyId }],
    },
    {
        name: "useMarkActionItem sends the item's new status",
        use: useMarkActionItem,
        variables: { itemId: "i1", status: "done" },
        endpoint: endpoints.markActionItem,
        request: [BOARD, "i1", { op_id: anyId, status: "done" }],
    },
    {
        name: "useDeleteActionItem deletes the item",
        use: useDeleteActionItem,
        variables: { itemId: "i1" },
        endpoint: endpoints.deleteActionItem,
        request: [BOARD, "i1", { op_id: anyId }],
    },
    {
        name: "useReviewPreviousAction sends the review's outcome",
        use: useReviewPreviousAction,
        variables: { itemId: "p1", outcome: "carried" },
        endpoint: endpoints.reviewPreviousAction,
        request: [BOARD, "p1", { op_id: anyId, outcome: "carried" }],
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

describe("action item commands", () => {
    it.each(commands)(
        "$name, leaves the board for its own frame and marks the listings stale",
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
            expectInvalidated(qc, queryKeys.actionItems.all);
            expectInvalidated(qc, queryKeys.auth.mentions());
            expect(invalidate).toHaveBeenCalledTimes(2);
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

    it.each([403, 404, 409, 500])("refetches the board after a %i", async status => {
        // given
        endpoints.markActionItem.mockRejectedValue(makeApiError({ status, message: "changed meanwhile" }));
        const { result, qc, invalidate } = renderCommand(useMarkActionItem);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ itemId: "i1", status: "done" })).rejects.toThrow(
                "changed meanwhile",
            );
        });

        // then
        expectInvalidated(qc, BOARD_STATE);
        expect(invalidate).toHaveBeenCalledTimes(1);
    });

    it("leaves everything alone when the title is refused", async () => {
        // given
        endpoints.addActionItem.mockRejectedValue(
            makeApiError({ status: 400, message: "an action item needs a title", code: "action_item_title_required" }),
        );
        const { result, invalidate } = renderCommand(useAddActionItem);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({
                    title: "",
                    assignee_user_id: null,
                    due_on: null,
                    mentions: [],
                    source_card_id: "k1",
                }),
            ).rejects.toThrow("an action item needs a title");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("leaves everything alone when the card is refused", async () => {
        // given
        endpoints.addActionItem.mockRejectedValue(
            makeApiError({ status: 400, message: "an action item is made from a card", code: "card_required" }),
        );
        const { result, invalidate } = renderCommand(useAddActionItem);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({
                    title: "Book the room",
                    assignee_user_id: null,
                    due_on: null,
                    mentions: [],
                    source_card_id: "",
                }),
            ).rejects.toThrow("an action item is made from a card");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("gives every new item its own id", async () => {
        // given
        const { result } = renderCommand(useAddActionItem);
        const draft = { title: "One", assignee_user_id: null, due_on: null, mentions: [], source_card_id: "k1" };

        // when
        await act(async () => {
            await result.current.mutateAsync(draft);
            await result.current.mutateAsync(draft);
        });

        // then
        const ids = endpoints.addActionItem.mock.calls.map(call => (call[1] as { id: string }).id);
        expect(ids[0]).not.toBe(ids[1]);
    });

    it("hands back the server's answer with the item", async () => {
        // given
        const answer = makeBoardCommandResponse({ version: 9, action_item: makeActionItem() });
        endpoints.addActionItem.mockResolvedValue(answer);
        const { result } = renderCommand(useAddActionItem);
        const draft = { title: "One", assignee_user_id: null, due_on: null, mentions: [], source_card_id: "k1" };

        // when
        let added: unknown;
        await act(async () => {
            added = await result.current.mutateAsync(draft);
        });

        // then
        expect(added).toEqual(answer);
    });
});

describe("useMarkListedActionItem", () => {
    function renderListed() {
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const rendered = renderHook(() => useMarkListedActionItem("o1"), {
            wrapper: providerWrapper({ queryClient: qc }),
        });

        return { ...rendered, qc, invalidate };
    }

    it("marks the item through its own board, then refreshes the page, the mentions and that board", async () => {
        // given
        const { result, qc, invalidate } = renderListed();

        // when
        await act(async () => {
            await result.current.mutateAsync({ boardId: "b7", itemId: "i1", status: "done" });
        });

        // then
        expect(endpoints.markActionItem).toHaveBeenCalledWith("b7", "i1", { op_id: anyId, status: "done" });
        expectInvalidated(qc, queryKeys.actionItems.organisation("o1"));
        expectInvalidated(qc, queryKeys.auth.mentions());
        expectInvalidated(qc, queryKeys.boardLive.state("b7"));
        expect(invalidate).toHaveBeenCalledTimes(3);
    });

    it("refreshes the page after a refusal that means the item changed, and not after one that changed nothing", async () => {
        // given
        const { result, invalidate } = renderListed();
        endpoints.markActionItem.mockRejectedValueOnce(makeApiError({ status: 409, message: "read only" }));
        endpoints.markActionItem.mockRejectedValueOnce(makeApiError({ status: 429, message: "slow down" }));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ boardId: "b7", itemId: "i1", status: "done" })).rejects.toThrow(
                "read only",
            );
        });

        // then
        expect(invalidate).toHaveBeenCalledTimes(3);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ boardId: "b7", itemId: "i1", status: "done" })).rejects.toThrow(
                "slow down",
            );
        });

        // then
        expect(invalidate).toHaveBeenCalledTimes(3);
    });
});
