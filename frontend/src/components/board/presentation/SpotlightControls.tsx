import { boardActions } from "../../../domain/board/capabilities";
import { facilitationRefusal } from "../../../domain/board/facilitation";
import { presentationOrder } from "../../../domain/board/presentation";
import { compareByPosition } from "../../../domain/board/sort";
import { useFocusColumn, useHighlightCard, usePresentCard } from "../../../hooks/mutations/facilitation";
import type { BoardSnapshot } from "../../../types/api";
import { Button } from "../../Button/Button";
import { ErrorBanner } from "../../ErrorBanner/ErrorBanner";
import { Select } from "../../Select/Select";
import styles from "./Presentation.module.css";

interface SpotlightControlsProps {
    snapshot: BoardSnapshot;
    writable: boolean;
}

const ALL_COLUMNS = "";

export function SpotlightControls({ snapshot, writable }: SpotlightControlsProps) {
    const present = usePresentCard(snapshot.board_id);
    const focus = useFocusColumn(snapshot.board_id);
    const highlight = useHighlightCard(snapshot.board_id);

    if (!writable || !boardActions(snapshot).spotlight) {
        return null;
    }

    const { spotlight } = snapshot;
    const order = presentationOrder(snapshot);
    const first = order.at(0);
    const focusable = snapshot.columns.toSorted(compareByPosition).filter(column => !column.hidden);
    const failure = present.error ?? focus.error ?? highlight.error;

    return (
        <div className={styles.controls} role="region" aria-label="Spotlight">
            {spotlight.presenting_card_id === null && (
                <div className={styles.controlGroup}>
                    <Button
                        type="button"
                        size="small"
                        variant="primary"
                        disabled={first === undefined || present.isPending}
                        onClick={() => {
                            focus.reset();
                            highlight.reset();
                            present.mutate({ cardId: first ?? null });
                        }}
                    >
                        Present
                    </Button>
                    {first === undefined && <span className={styles.note}>Reveal some cards first</span>}
                </div>
            )}
            <Select
                size="small"
                aria-label="Focus column"
                className={styles.select}
                value={spotlight.focused_column_id ?? ALL_COLUMNS}
                disabled={focus.isPending}
                onChange={e => {
                    present.reset();
                    highlight.reset();
                    focus.mutate({ columnId: e.target.value === ALL_COLUMNS ? null : e.target.value });
                }}
            >
                <option value={ALL_COLUMNS}>All columns</option>
                {focusable.map(column => (
                    <option key={column.id} value={column.id}>
                        {column.title}
                    </option>
                ))}
            </Select>
            {spotlight.highlighted_card_id !== null && (
                <Button
                    type="button"
                    size="small"
                    disabled={highlight.isPending}
                    onClick={() => {
                        present.reset();
                        focus.reset();
                        highlight.mutate({ cardId: null });
                    }}
                >
                    Clear highlight
                </Button>
            )}
            {failure !== null && (
                <div className={styles.failure}>
                    <ErrorBanner message={facilitationRefusal(failure)} />
                </div>
            )}
        </div>
    );
}
