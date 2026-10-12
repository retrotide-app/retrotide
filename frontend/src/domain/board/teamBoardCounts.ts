export interface TeamScopedBoard {
    team_id?: string;
}

export function boardCountsByTeam(boards: readonly TeamScopedBoard[]): ReadonlyMap<string, number> {
    const counts = new Map<string, number>();

    for (const board of boards) {
        if (board.team_id === undefined) {
            continue;
        }

        counts.set(board.team_id, (counts.get(board.team_id) ?? 0) + 1);
    }

    return counts;
}
