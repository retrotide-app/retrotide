import { type ReactNode, useId } from "react";
import styles from "./MarketingSection.module.css";

interface MarketingSectionProps {
    title: string;
    intro?: ReactNode;
    children?: ReactNode;
}

export function MarketingSection({ title, intro, children }: MarketingSectionProps) {
    const titleId = useId();

    return (
        <section className={styles.section} aria-labelledby={titleId}>
            <div className={styles.head}>
                <h2 id={titleId} className={styles.heading} dir="auto">
                    {title}
                </h2>
                {intro !== undefined && <p className={styles.intro}>{intro}</p>}
            </div>
            {children}
        </section>
    );
}
