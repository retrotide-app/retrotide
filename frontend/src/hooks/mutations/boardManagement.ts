import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
    archiveBoard,
    bulkBoards,
    deleteBoard,
    importCsv,
    moveBoard,
    pinBoard,
    restoreBoard,
    unarchiveBoard,
    unpinBoard,
} from "../../api/endpoints/boardManagement";
import { queryKeys } from "../../api/queryKeys";
import type { BulkBoardAction } from "../../types/api";
import { BOARD_OUTDATED_ON, type CommandOptions, GONE_STATUS, useBoardCommand } from "./boardCommand";
import { newId } from "./ids";
import { refreshWhenChanged } from "./refresh";

export interface BoardMove {
    teamId: string | null;
}

export interface BoardPinChange {
    boardId: string;
    pinned: boolean;
}

export interface BulkBoardChange {
    action: BulkBoardAction;
    boardIds: string[];
}

const BOARD_LISTS_CHANGED: CommandOptions = {
    answered: qc => qc.invalidateQueries({ queryKey: queryKeys.boards.all }),
};

export function useArchiveBoard(boardId: string) {
    return useBoardCommand(
        boardId,
        (_variables: void, opId) => archiveBoard(boardId, { op_id: opId }),
        BOARD_LISTS_CHANGED,
    );
}

export function useUnarchiveBoard(boardId: string) {
    return useBoardCommand(
        boardId,
        (_variables: void, opId) => unarchiveBoard(boardId, { op_id: opId }),
        BOARD_LISTS_CHANGED,
    );
}

export function useDeleteBoard(boardId: string, orgId: string) {
    return useBoardCommand(boardId, (_variables: void, opId) => deleteBoard(boardId, { op_id: opId }), {
        answered: qc =>
            Promise.all([
                qc.invalidateQueries({ queryKey: queryKeys.boards.all }),
                qc.invalidateQueries({ queryKey: queryKeys.organisations.deletedBoards(orgId) }),
            ]),
    });
}

export function useMoveBoard(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ teamId }: BoardMove, opId) => moveBoard(boardId, { op_id: opId, team_id: teamId }),
        BOARD_LISTS_CHANGED,
    );
}

export function useImportCsv(boardId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationKey: queryKeys.boardLive.mutations(boardId),
        mutationFn: (csv: string) => importCsv(boardId, { op_id: newId(), csv }),
        onError: error => refreshWhenChanged(qc, error, [queryKeys.boardLive.state(boardId)], BOARD_OUTDATED_ON),
    });
}

export function usePinBoard(orgId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ boardId, pinned }: BoardPinChange) => (pinned ? pinBoard(boardId) : unpinBoard(boardId)),
        onSettled: (_data, error) => refreshWhenChanged(qc, error, [queryKeys.boards.organisation(orgId)]),
    });
}

export function useRestoreBoard(orgId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: (boardId: string) => restoreBoard(orgId, boardId),
        onSettled: (_data, error) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.boards.organisation(orgId), queryKeys.organisations.deletedBoards(orgId)],
                [GONE_STATUS],
            ),
    });
}

export function useBulkBoards(orgId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ action, boardIds }: BulkBoardChange) => bulkBoards(orgId, { action, board_ids: boardIds }),
        onSettled: (_data, error) =>
            refreshWhenChanged(qc, error, [
                queryKeys.boards.organisation(orgId),
                queryKeys.organisations.deletedBoards(orgId),
            ]),
    });
}
