import type { BoardSnapshot } from "../../types/api";

export function keepNewerSnapshot(held: BoardSnapshot | undefined, incoming: BoardSnapshot): BoardSnapshot {
    if (held !== undefined && held.board_id === incoming.board_id && held.version > incoming.version) {
        return held;
    }

    return incoming;
}
