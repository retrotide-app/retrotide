import { type ReactNode, useId } from "react";
import styles from "./Settings.module.css";

export type SettingsHeadingLevel = 2 | 3;

interface SettingsRowProps {
    title: ReactNode;
    headingLevel: SettingsHeadingLevel;
    description?: ReactNode;
    tone?: "default" | "danger";
    children?: ReactNode;
}

export function SettingsRow({ title, headingLevel, description, tone = "default", children }: SettingsRowProps) {
    const titleId = useId();
    const titleClass = tone === "danger" ? `${styles.title} ${styles.danger}` : styles.title;

    return (
        <section aria-labelledby={titleId} className={styles.row}>
            <div className={styles.text}>
                {headingLevel === 2 ? (
                    <h2 id={titleId} className={titleClass} dir="auto">
                        {title}
                    </h2>
                ) : (
                    <h3 id={titleId} className={titleClass} dir="auto">
                        {title}
                    </h3>
                )}
                {description !== undefined && <p className={styles.description}>{description}</p>}
            </div>
            <div className={styles.control}>{children}</div>
        </section>
    );
}
