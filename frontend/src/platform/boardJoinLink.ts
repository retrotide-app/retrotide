import { reportPlatformError } from "./errorReporter";

export const BOARD_JOIN_LINK_STORAGE_PREFIX = "rt-join-link:";

export function readBoardJoinLink(boardId: string): string | null {
    let stored: string | null;

    try {
        stored = sessionStorage.getItem(BOARD_JOIN_LINK_STORAGE_PREFIX + boardId);
    } catch (error) {
        reportPlatformError(error, "storage");
        return null;
    }

    if (stored === null || stored === "") {
        return null;
    }

    return stored;
}

export function writeBoardJoinLink(boardId: string, slug: string): void {
    try {
        sessionStorage.setItem(BOARD_JOIN_LINK_STORAGE_PREFIX + boardId, slug);
    } catch (error) {
        reportPlatformError(error, "storage");
    }
}
