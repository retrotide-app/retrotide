import type { BoardRefusalReason } from "../../types/api";

export type BoardSyncStatus =
    | { state: "connecting" }
    | { state: "live" }
    | { state: "refused"; reason: BoardRefusalReason; revoked: boolean };

export const SYNC_CONNECTING: BoardSyncStatus = { state: "connecting" };

export const SYNC_LIVE: BoardSyncStatus = { state: "live" };

const REFUSAL_NOTICES: Record<BoardRefusalReason, string | null> = {
    not_participant: "You no longer have access to this board, so it has stopped updating.",
    password_required: "This board needs its password again before it can update live.",
    account_required: "Sign in to see this board update live.",
    link_disabled: "The link to this board has been turned off, so it has stopped updating.",
    removed: "A facilitator removed you from this board, so it has stopped updating.",
    archived: null,
};

export function refusalNotice(status: BoardSyncStatus): string | null {
    if (status.state !== "refused") {
        return null;
    }

    return REFUSAL_NOTICES[status.reason];
}
