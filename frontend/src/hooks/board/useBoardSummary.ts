import { useMemo } from "react";
import { type BoardSummary, boardSummary } from "../../domain/board/summary";
import { useBoard } from "../queries/boards";
import type { QueryState } from "../queries/queryState";

export function useBoardSummary(boardId: string): QueryState<BoardSummary> {
    const board = useBoard(boardId);
    const snapshot = board.data;

    const summary = useMemo(() => (snapshot === undefined ? undefined : boardSummary(snapshot)), [snapshot]);

    return { ...board, data: summary };
}
