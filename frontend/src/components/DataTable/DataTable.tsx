import type { ReactNode } from "react";
import styles from "./DataTable.module.css";

interface DataTableProps {
    caption: string;
    children: ReactNode;
}

export function DataTable({ caption, children }: DataTableProps) {
    return (
        <div className={styles.scroll} role="region" aria-label={caption} tabIndex={0}>
            <table className={styles.table}>
                <caption className="visually-hidden">{caption}</caption>
                {children}
            </table>
        </div>
    );
}
