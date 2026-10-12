import { type ReactNode, useId } from "react";
import styles from "./PageState.module.css";

export type PageStateHeadingLevel = 1 | 2;

interface PageStateProps {
    title: string;
    headingLevel?: PageStateHeadingLevel;
    description?: ReactNode;
    children?: ReactNode;
}

export function PageState({ title, headingLevel = 1, description, children }: PageStateProps) {
    const titleId = useId();

    return (
        <section className={styles.state} aria-labelledby={titleId}>
            {headingLevel === 1 ? (
                <h1 id={titleId} className={styles.title} dir="auto">
                    {title}
                </h1>
            ) : (
                <h2 id={titleId} className={styles.title} dir="auto">
                    {title}
                </h2>
            )}
            {description !== undefined && <p className={styles.description}>{description}</p>}
            {children}
        </section>
    );
}
