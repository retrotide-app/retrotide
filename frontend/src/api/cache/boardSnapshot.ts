import { type QueryClient, queryOptions, replaceEqualDeep } from "@tanstack/react-query";
import { keepNewerSnapshot } from "../../domain/board/snapshot";
import type { BoardSnapshot } from "../../types/api";
import { type BoardFetchReason, getBoard } from "../endpoints/boards";
import { queryKeys } from "../queryKeys";

async function fetchBoardSnapshot(qc: QueryClient, boardId: string, reason?: BoardFetchReason): Promise<BoardSnapshot> {
    const held = qc.getQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId));
    const fresh = await getBoard(boardId, held?.version, reason);

    if (fresh !== undefined) {
        return fresh;
    }

    if (held === undefined) {
        throw new Error(`board ${boardId} answered unchanged before any version of it was held`);
    }

    return held;
}

function shareNewerSnapshot(held: unknown, incoming: unknown): unknown {
    const kept = keepNewerSnapshot(held as BoardSnapshot | undefined, incoming as BoardSnapshot);

    if (kept === held) {
        return held;
    }

    return replaceEqualDeep(held, incoming);
}

export function boardSnapshotQuery(qc: QueryClient, boardId: string, reason?: BoardFetchReason) {
    return queryOptions({
        queryKey: queryKeys.boardLive.state(boardId),
        queryFn: () => fetchBoardSnapshot(qc, boardId, reason),
        structuralSharing: shareNewerSnapshot,
    });
}

export async function catchUpBoard(qc: QueryClient, boardId: string): Promise<void> {
    await qc.query({ ...boardSnapshotQuery(qc, boardId, "reconnect"), staleTime: 0 });
}

export async function refreshPreviousActions(qc: QueryClient, boardId: string): Promise<void> {
    const fresh = await getBoard(boardId);

    if (fresh === undefined) {
        throw new Error(`board ${boardId} answered unchanged to a read that sent no version`);
    }

    const held = qc.getQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId));

    if (held === undefined) {
        return;
    }

    if (keepNewerSnapshot(held, fresh) === held) {
        await refreshPreviousActions(qc, boardId);
        return;
    }

    if (held.board_id === fresh.board_id && held.version === fresh.version) {
        qc.setQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId), {
            ...held,
            previous_actions: fresh.previous_actions,
        });
        return;
    }

    qc.setQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId), replaceEqualDeep(held, fresh));
}
