import { reportPlatformError } from "./errorReporter";

export const PENDING_INVITE_STORAGE_KEY = "rt-pending-invite";

export function readPendingInvite(): string | null {
    let stored: string | null;

    try {
        stored = sessionStorage.getItem(PENDING_INVITE_STORAGE_KEY);
    } catch (error) {
        reportPlatformError(error, "storage");
        return null;
    }

    if (stored === null || stored === "") {
        return null;
    }

    return stored;
}

export function writePendingInvite(token: string): void {
    try {
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, token);
    } catch (error) {
        reportPlatformError(error, "storage");
    }
}

export function clearPendingInvite(): void {
    try {
        sessionStorage.removeItem(PENDING_INVITE_STORAGE_KEY);
    } catch (error) {
        reportPlatformError(error, "storage");
    }
}
