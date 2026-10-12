import type { QueryKey } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeBoardCommandResponse } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useArchiveBoard,
    useBulkBoards,
    useDeleteBoard,
    useImportCsv,
    useMoveBoard,
    usePinBoard,
    useRestoreBoard,
    useUnarchiveBoard,
} from "./boardManagement";

const endpoints = vi.hoisted(() => ({
    archiveBoard: vi.fn(),
    unarchiveBoard: vi.fn(),
    deleteBoard: vi.fn(),
    moveBoard: vi.fn(),
    importCsv: vi.fn(),
    pinBoard: vi.fn(),
    unpinBoard: vi.fn(),
    restoreBoard: vi.fn(),
    bulkBoards: vi.fn(),
}));

vi.mock("../../api/endpoints/boardManagement", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const anyId = expect.stringMatching(V4);

const BOARD = "b1";

const ORG = "o1";

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
    refreshes: QueryKey[];
}

const commands: CommandCase[] = [
    {
        name: "useArchiveBoard archives the board",
        use: useArchiveBoard,
        variables: undefined,
        endpoint: endpoints.archiveBoard,
        request: [BOARD, { op_id: anyId }],
        refreshes: [queryKeys.boards.all],
    },
    {
        name: "useUnarchiveBoard brings the board back",
        use: useUnarchiveBoard,
        variables: undefined,
        endpoint: endpoints.unarchiveBoard,
        request: [BOARD, { op_id: anyId }],
        refreshes: [queryKeys.boards.all],
    },
    {
        name: "useDeleteBoard deletes the board",
        use: boardId => useDeleteBoard(boardId, ORG),
        variables: undefined,
        endpoint: endpoints.deleteBoard,
        request: [BOARD, { op_id: anyId }],
        refreshes: [queryKeys.boards.all, queryKeys.organisations.deletedBoards(ORG)],
    },
    {
        name: "useMoveBoard moves the board to a team",
        use: useMoveBoard,
        variables: { teamId: "t2" },
        endpoint: endpoints.moveBoard,
        request: [BOARD, { op_id: anyId, team_id: "t2" }],
        refreshes: [queryKeys.boards.all],
    },
    {
        name: "useMoveBoard moves the board to the organisation",
        use: useMoveBoard,
        variables: { teamId: null },
        endpoint: endpoints.moveBoard,
        request: [BOARD, { op_id: anyId, team_id: null }],
        refreshes: [queryKeys.boards.all],
    },
];

function renderWithClient<T>(use: () => T) {
    const qc = createTestQueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    const rendered = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

    return { ...rendered, qc, invalidate };
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
        fn.mockResolvedValue(makeBoardCommandResponse());
    }
});

describe("board management commands", () => {
    it.each(commands)(
        "$name, and refreshes the board lists",
        async ({ use, variables, endpoint, request, refreshes }) => {
            // given
            const { result, invalidate } = renderWithClient(() => use(BOARD));

            // when
            await act(async () => {
                await result.current.mutateAsync(variables as never);
            });

            // then
            expect(endpoint).toHaveBeenCalledTimes(1);
            expect(endpoint.mock.calls[0]).toEqual(request);
            expect(invalidate.mock.calls).toEqual(refreshes.map(queryKey => [{ queryKey }]));
        },
    );

    it.each(commands)("files the command under the board's mutation key: $name", async ({ use, variables }) => {
        // given
        const { result, qc } = renderWithClient(() => use(BOARD));

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
        });

        // then
        expect(qc.getMutationCache().findAll({ mutationKey: queryKeys.boardLive.mutations(BOARD) })).toHaveLength(1);
    });

    it("refetches the board after a refusal that means it changed", async () => {
        // given
        endpoints.archiveBoard.mockRejectedValue(
            makeApiError({ status: 403, message: "your role on this board does not allow that" }),
        );
        const { result, qc } = renderWithClient(() => useArchiveBoard(BOARD));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync()).rejects.toThrow("your role on this board does not allow that");
        });

        // then
        expectInvalidated(qc, queryKeys.boardLive.state(BOARD));
    });
});

describe("useImportCsv", () => {
    it("posts the CSV with a fresh op id and hands back the counts", async () => {
        // given
        endpoints.importCsv.mockResolvedValue({ version: 9, columns_created: 1, cards_created: 4 });
        const { result, qc } = renderWithClient(() => useImportCsv(BOARD));

        // when
        let answer: unknown;
        await act(async () => {
            answer = await result.current.mutateAsync("Went well\nShipped\n");
        });

        // then
        expect(endpoints.importCsv).toHaveBeenCalledWith(BOARD, { op_id: anyId, csv: "Went well\nShipped\n" });
        expect(answer).toEqual({ version: 9, columns_created: 1, cards_created: 4 });
        expect(qc.getMutationCache().findAll({ mutationKey: queryKeys.boardLive.mutations(BOARD) })).toHaveLength(1);
    });

    it("leaves the board alone when the CSV is refused", async () => {
        // given
        endpoints.importCsv.mockRejectedValue(
            makeApiError({ status: 400, message: "the CSV holds no cards to import", code: "csv_empty" }),
        );
        const { result, invalidate } = renderWithClient(() => useImportCsv(BOARD));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync("Ideas\n")).rejects.toThrow("the CSV holds no cards to import");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("usePinBoard", () => {
    it.each([
        { pinned: true, endpoint: endpoints.pinBoard },
        { pinned: false, endpoint: endpoints.unpinBoard },
    ])("pins or unpins the board ($pinned) and refreshes the organisation's boards", async ({ pinned, endpoint }) => {
        // given
        endpoint.mockResolvedValue(undefined);
        const { result, qc } = renderWithClient(() => usePinBoard(ORG));

        // when
        await act(async () => {
            await result.current.mutateAsync({ boardId: BOARD, pinned });
        });

        // then
        expect(endpoint).toHaveBeenCalledWith(BOARD);
        expectInvalidated(qc, queryKeys.boards.organisation(ORG));
    });
});

describe("useRestoreBoard", () => {
    it("restores the board and refreshes the board list and the deleted list", async () => {
        // given
        endpoints.restoreBoard.mockResolvedValue(undefined);
        const { result, qc } = renderWithClient(() => useRestoreBoard(ORG));

        // when
        await act(async () => {
            await result.current.mutateAsync(BOARD);
        });

        // then
        expect(endpoints.restoreBoard).toHaveBeenCalledWith(ORG, BOARD);
        expectInvalidated(qc, queryKeys.boards.organisation(ORG));
        expectInvalidated(qc, queryKeys.organisations.deletedBoards(ORG));
    });

    it("refreshes the deleted list when the board is past its window", async () => {
        // given
        endpoints.restoreBoard.mockRejectedValue(makeApiError({ status: 404, message: "board not found" }));
        const { result, qc } = renderWithClient(() => useRestoreBoard(ORG));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(BOARD)).rejects.toThrow("board not found");
        });

        // then
        expectInvalidated(qc, queryKeys.organisations.deletedBoards(ORG));
    });

    it("leaves the lists alone after a refusal that changed nothing", async () => {
        // given
        endpoints.restoreBoard.mockRejectedValue(
            makeApiError({ status: 403, message: "you cannot restore that board", code: "restore_not_allowed" }),
        );
        const { result, invalidate } = renderWithClient(() => useRestoreBoard(ORG));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(BOARD)).rejects.toThrow("you cannot restore that board");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useBulkBoards", () => {
    it("sends the action for every chosen board and hands back each result", async () => {
        // given
        const answer = {
            results: [
                { board_id: "b1", ok: true },
                { board_id: "b2", ok: false, error: "board not found" },
            ],
        };
        endpoints.bulkBoards.mockResolvedValue(answer);
        const { result, qc } = renderWithClient(() => useBulkBoards(ORG));

        // when
        let got: unknown;
        await act(async () => {
            got = await result.current.mutateAsync({ action: "delete", boardIds: ["b1", "b2"] });
        });

        // then
        expect(endpoints.bulkBoards).toHaveBeenCalledWith(ORG, { action: "delete", board_ids: ["b1", "b2"] });
        expect(got).toEqual(answer);
        expectInvalidated(qc, queryKeys.boards.organisation(ORG));
        expectInvalidated(qc, queryKeys.organisations.deletedBoards(ORG));
    });
});
