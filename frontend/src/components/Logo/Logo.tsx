import styles from "./Logo.module.css";

interface LogoProps {
    label?: string;
    className?: string;
}

export function Logo({ label, className }: LogoProps) {
    const classes = [styles.logo, className].filter(Boolean).join(" ");
    const labelled = label !== undefined;

    return (
        <span
            className={classes}
            role={labelled ? "img" : undefined}
            aria-label={label}
            aria-hidden={labelled ? undefined : true}
        >
            <img src="/logo.png" alt="" width={1448} height={1086} className={styles.light} />
            <img src="/logo-dark.png" alt="" width={1448} height={1086} className={styles.dark} />
        </span>
    );
}
