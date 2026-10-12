import { type ReactNode, useEffect, useId, useRef } from "react";
import styles from "./Panel.module.css";

export type PanelHeadingLevel = 2 | 3;

interface PanelProps {
    title: ReactNode;
    headingLevel: PanelHeadingLevel;
    description?: ReactNode;
    tone?: "default" | "danger";
    takeFocus?: boolean;
    children?: ReactNode;
}

export function Panel({ title, headingLevel, description, tone = "default", takeFocus = false, children }: PanelProps) {
    const titleId = useId();
    const headingRef = useRef<HTMLHeadingElement>(null);
    const classes = tone === "danger" ? `${styles.panel} ${styles.danger}` : styles.panel;
    const tabIndex = takeFocus ? -1 : undefined;

    useEffect(() => {
        if (!takeFocus) {
            return;
        }

        headingRef.current?.focus();
    }, [takeFocus]);

    return (
        <section aria-labelledby={titleId} className={classes}>
            {headingLevel === 2 ? (
                <h2 ref={headingRef} id={titleId} className={styles.title} dir="auto" tabIndex={tabIndex}>
                    {title}
                </h2>
            ) : (
                <h3 ref={headingRef} id={titleId} className={styles.title} dir="auto" tabIndex={tabIndex}>
                    {title}
                </h3>
            )}
            {description !== undefined && <p className={styles.description}>{description}</p>}
            {children}
        </section>
    );
}
