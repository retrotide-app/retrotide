import type { HiddenPlaceholder } from "../../domain/board/selectors";
import { useArrival } from "../../hooks/board/useArrivals";
import styles from "./HiddenCards.module.css";

interface HiddenCardsProps {
    placeholders: readonly HiddenPlaceholder[];
}

interface PlaceholderProps {
    placeholderKey: string;
}

function Placeholder({ placeholderKey }: PlaceholderProps) {
    const arriving = useArrival(placeholderKey);

    return <li className={arriving ? `${styles.placeholder} ${styles.arriving}` : styles.placeholder} />;
}

function hiddenSummary(count: number): string {
    return count === 1 ? "1 card from someone else, hidden for now" : `${count} cards from others, hidden for now`;
}

export function HiddenCards({ placeholders }: HiddenCardsProps) {
    if (placeholders.length === 0) {
        return null;
    }

    return (
        <div className={styles.hidden}>
            <p className={styles.summary}>{hiddenSummary(placeholders.length)}</p>
            <ul className={styles.list} aria-hidden="true">
                {placeholders.map(placeholder => (
                    <Placeholder key={placeholder.key} placeholderKey={placeholder.key} />
                ))}
            </ul>
        </div>
    );
}
