import type { TemplateColumnResponse } from "../../../types/api";
import colours from "../../board/boardColours.module.css";
import styles from "./TemplateColumns.module.css";

interface TemplateColumnsProps {
    columns: readonly TemplateColumnResponse[];
    label: string;
}

export function TemplateColumns({ columns, label }: TemplateColumnsProps) {
    return (
        <ol className={styles.columns} aria-label={label}>
            {columns.map(column => (
                <li key={column.title} className={`${styles.column} ${colours[column.colour]}`}>
                    <h3 className={styles.title} dir="auto">
                        {column.title}
                    </h3>
                    <p className={styles.description} dir="auto">
                        {column.description}
                    </p>
                    <span className={styles.placeholder} aria-hidden="true" />
                    <span className={styles.placeholder} aria-hidden="true" />
                </li>
            ))}
        </ol>
    );
}
