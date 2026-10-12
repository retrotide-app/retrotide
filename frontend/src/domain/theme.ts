import type { ThemePreference } from "../types/app";

export const THEME_PREFERENCES: readonly ThemePreference[] = ["system", "light", "dark"];

export function isThemePreference(value: string): value is ThemePreference {
    return THEME_PREFERENCES.some(preference => preference === value);
}
