import { createContext } from "react";
import type { ThemePreference } from "../types/app";

export interface ThemeContextValue {
    preference: ThemePreference;
    setPreference: (next: ThemePreference) => void;
}

export const ThemeContext = createContext<ThemeContextValue | null>(null);
