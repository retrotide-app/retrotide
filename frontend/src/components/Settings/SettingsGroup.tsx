import { type ReactNode, useId } from "react";
import styles from "./Settings.module.css";

interface SettingsGroupProps {
    title?: string;
    children: ReactNode;
}

export function SettingsGroup({ title, children }: SettingsGroupProps) {
    const titleId = useId();

    if (title === undefined) {
        return <div className={styles.group}>{children}</div>;
    }

    return (
        <section aria-labelledby={titleId} className={styles.titled}>
            <h2 id={titleId} className={styles.groupTitle} dir="auto">
                {title}
            </h2>
            <div className={styles.group}>{children}</div>
        </section>
    );
}
