import { Button } from "../Button/Button";
import styles from "./Pagination.module.css";

interface PaginationProps {
    label: string;
    offset: number;
    limit: number;
    total: number;
    onChange: (offset: number) => void;
}

export function Pagination({ label, offset, limit, total, onChange }: PaginationProps) {
    if (limit <= 0 || (offset === 0 && total <= limit)) {
        return null;
    }

    const first = Math.min(offset + 1, total);
    const last = Math.min(offset + limit, total);
    const hasPrevious = offset > 0;
    const hasNext = offset + limit < total;

    return (
        <nav aria-label={label} className={styles.pagination}>
            <Button
                type="button"
                size="small"
                onClick={() => onChange(Math.max(0, offset - limit))}
                disabled={!hasPrevious}
            >
                Previous
            </Button>
            <span className={styles.summary}>
                {first} to {last} of {total}
            </span>
            <Button type="button" size="small" onClick={() => onChange(offset + limit)} disabled={!hasNext}>
                Next
            </Button>
        </nav>
    );
}
