import type { ChangeEvent } from "react";
import { isThemePreference, THEME_PREFERENCES } from "../../../domain/theme";
import { useTheme } from "../../../hooks/useTheme";
import type { ThemePreference } from "../../../types/app";
import { Select } from "../../Select/Select";
import styles from "./ThemeSelect.module.css";

const THEME_LABELS: Record<ThemePreference, string> = {
    system: "System",
    light: "Light",
    dark: "Dark",
};

export function ThemeSelect() {
    const { preference, setPreference } = useTheme();

    const handleChange = (event: ChangeEvent<HTMLSelectElement>) => {
        const next = event.target.value;

        if (!isThemePreference(next)) {
            throw new Error(`unknown theme preference "${next}"`);
        }

        setPreference(next);
    };

    return (
        <label className={styles.field}>
            <span className={styles.label}>Theme</span>
            <Select size="small" className={styles.select} value={preference} onChange={handleChange}>
                {THEME_PREFERENCES.map(option => (
                    <option key={option} value={option}>
                        {THEME_LABELS[option]}
                    </option>
                ))}
            </Select>
        </label>
    );
}
