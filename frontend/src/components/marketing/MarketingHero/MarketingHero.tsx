import { type ReactNode, useId } from "react";
import styles from "./MarketingHero.module.css";

interface MarketingHeroProps {
    title: string;
    lede: ReactNode;
    eyebrow?: string;
    children?: ReactNode;
}

export function MarketingHero({ title, lede, eyebrow, children }: MarketingHeroProps) {
    const titleId = useId();

    return (
        <section className={styles.hero} aria-labelledby={titleId}>
            <div className={styles.inner}>
                {eyebrow !== undefined && <p className={styles.eyebrow}>{eyebrow}</p>}
                <h1 id={titleId} className={styles.title} dir="auto">
                    {title}
                </h1>
                <p className={styles.lede} dir="auto">
                    {lede}
                </p>
                {children !== undefined && <div className={styles.actions}>{children}</div>}
            </div>
            <svg className={styles.wave} viewBox="0 0 1440 64" preserveAspectRatio="none" aria-hidden="true">
                <path d="M0 40C180 8 360 8 540 32s360 32 540 8 270-28 360-16V64H0z" />
            </svg>
        </section>
    );
}
