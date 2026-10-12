export const NEW_BOARD_TEAM_PARAM = "team";

export const NEW_BOARD_NO_TEAM = "none";

export const JOIN_BOARD_ROUTE = "/b/:slug";

export const SHARED_BOARD_ROUTE = "/boards/:boardId";

export const SHARED_BOARD_SUMMARY_ROUTE = `${SHARED_BOARD_ROUTE}/summary`;

export function boardPath(orgSlug: string, boardId: string): string {
    return `/o/${encodeURIComponent(orgSlug)}/boards/${encodeURIComponent(boardId)}`;
}

export function sharedBoardPath(boardId: string): string {
    return `/boards/${encodeURIComponent(boardId)}`;
}

export function joinBoardPath(slug: string): string {
    return `/b/${encodeURIComponent(slug)}`;
}

export function boardSettingsPath(orgSlug: string, boardId: string): string {
    return `${boardPath(orgSlug, boardId)}/settings`;
}

export function boardSummaryPath(orgSlug: string, boardId: string): string {
    return `${boardPath(orgSlug, boardId)}/summary`;
}

export function sharedBoardSummaryPath(boardId: string): string {
    return `${sharedBoardPath(boardId)}/summary`;
}

export function newBoardPath(orgSlug: string, teamId?: string): string {
    const path = `/o/${encodeURIComponent(orgSlug)}/boards/new`;

    if (teamId === undefined) {
        return path;
    }

    const query = new URLSearchParams({ [NEW_BOARD_TEAM_PARAM]: teamId });

    return `${path}?${query.toString()}`;
}
