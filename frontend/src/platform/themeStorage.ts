import { isThemePreference } from "../domain/theme";
import type { ThemePreference } from "../types/app";
import { reportPlatformError } from "./errorReporter";

export const THEME_STORAGE_KEY = "rt-theme";

export function readThemePreference(): ThemePreference | null {
    let stored: string | null;

    try {
        stored = localStorage.getItem(THEME_STORAGE_KEY);
    } catch (error) {
        reportPlatformError(error, "storage");
        return null;
    }

    if (stored === null || !isThemePreference(stored)) {
        return null;
    }

    return stored;
}

export function writeThemePreference(preference: ThemePreference): void {
    try {
        if (preference === "system") {
            localStorage.removeItem(THEME_STORAGE_KEY);
            return;
        }

        localStorage.setItem(THEME_STORAGE_KEY, preference);
    } catch (error) {
        reportPlatformError(error, "storage");
    }
}
