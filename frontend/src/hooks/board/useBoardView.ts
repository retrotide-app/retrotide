import { useMemo } from "react";
import { useBoardSync } from "../../api/realtime/sync/useBoardSync";
import { withPendingCommands } from "../../domain/board/optimistic";
import type { BoardSyncStatus } from "../../domain/board/syncStatus";
import type { BoardSnapshot } from "../../types/api";
import { usePendingBoardCommands } from "../mutations/boards";
import { useBoard } from "../queries/boards";
import type { QueryState } from "../queries/queryState";

export interface BoardViewState extends QueryState<BoardSnapshot> {
    sync: BoardSyncStatus;
}

export function useBoardView(boardId: string): BoardViewState {
    const board = useBoard(boardId);
    const pending = usePendingBoardCommands(boardId);
    const sync = useBoardSync(boardId);
    const confirmed = board.data;

    const shown = useMemo(
        () => (confirmed === undefined ? undefined : withPendingCommands(confirmed, pending)),
        [confirmed, pending],
    );

    return { ...board, data: shown, sync };
}
