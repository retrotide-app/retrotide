import type { ReactNode } from "react";
import styles from "./PageHeader.module.css";

interface PageHeaderProps {
    titleId: string;
    title: string;
    description?: ReactNode;
    headingLevel?: 1 | 2;
    badge?: ReactNode;
    actions?: ReactNode;
    className?: string;
}

export function PageHeader({
    titleId,
    title,
    description,
    headingLevel = 1,
    badge,
    actions,
    className,
}: PageHeaderProps) {
    const Heading = headingLevel === 1 ? "h1" : "h2";
    const classes = [styles.header, className].filter(Boolean).join(" ");

    return (
        <header className={classes}>
            <div className={styles.top}>
                <div className={styles.titleGroup}>
                    <Heading id={titleId} className={styles.title} dir="auto">
                        {title}
                    </Heading>
                    {badge}
                </div>
                {actions !== undefined && <div className={styles.actions}>{actions}</div>}
            </div>
            {description !== undefined && <p className={styles.description}>{description}</p>}
        </header>
    );
}
