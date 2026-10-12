import { type PropsWithChildren, useCallback, useLayoutEffect, useMemo, useState } from "react";
import { readThemePreference, writeThemePreference } from "../platform/themeStorage";
import type { ThemePreference } from "../types/app";
import { ThemeContext } from "./themeContextValue";

export function ThemeProvider({ children }: PropsWithChildren) {
    const [preference, setPreferenceState] = useState<ThemePreference>(() => readThemePreference() ?? "system");

    useLayoutEffect(() => {
        if (preference === "system") {
            document.documentElement.removeAttribute("data-theme");
            return;
        }

        document.documentElement.setAttribute("data-theme", preference);
    }, [preference]);

    const setPreference = useCallback((next: ThemePreference) => {
        setPreferenceState(next);
        writeThemePreference(next);
    }, []);

    const value = useMemo(() => ({ preference, setPreference }), [preference, setPreference]);

    return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
