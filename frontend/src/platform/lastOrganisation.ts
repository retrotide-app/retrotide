import { reportPlatformError } from "./errorReporter";

export const LAST_ORGANISATION_STORAGE_KEY = "rt-last-org";

export function readLastOrganisation(): string | null {
    let stored: string | null;

    try {
        stored = localStorage.getItem(LAST_ORGANISATION_STORAGE_KEY);
    } catch (error) {
        reportPlatformError(error, "storage");
        return null;
    }

    if (stored === null || stored === "") {
        return null;
    }

    return stored;
}

export function writeLastOrganisation(orgId: string): void {
    try {
        localStorage.setItem(LAST_ORGANISATION_STORAGE_KEY, orgId);
    } catch (error) {
        reportPlatformError(error, "storage");
    }
}
