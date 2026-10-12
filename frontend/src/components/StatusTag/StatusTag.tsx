import type { ReactNode } from "react";
import styles from "./StatusTag.module.css";

export type StatusTone = "neutral" | "success" | "warning" | "danger";

interface StatusTagProps {
    tone: StatusTone;
    children: ReactNode;
}

export function StatusTag({ tone, children }: StatusTagProps) {
    return <span className={`${styles.tag} ${styles[tone]}`}>{children}</span>;
}
