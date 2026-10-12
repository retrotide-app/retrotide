import { useId } from "react";
import { LoadFailure } from "../LoadFailure/LoadFailure";
import styles from "./SessionCheckFailure.module.css";

export type PageHeadingLevel = 1 | 2;

interface SessionCheckFailureProps {
    message: string;
    onRetry: () => unknown;
    headingLevel?: PageHeadingLevel;
}

const TITLE = "We could not check your sign-in";

export function SessionCheckFailure({ message, onRetry, headingLevel = 1 }: SessionCheckFailureProps) {
    const titleId = useId();

    return (
        <section className={styles.state} aria-labelledby={titleId}>
            {headingLevel === 1 ? (
                <h1 id={titleId} className={styles.title}>
                    {TITLE}
                </h1>
            ) : (
                <h2 id={titleId} className={styles.title}>
                    {TITLE}
                </h2>
            )}
            <LoadFailure message={message} onRetry={onRetry} />
        </section>
    );
}
