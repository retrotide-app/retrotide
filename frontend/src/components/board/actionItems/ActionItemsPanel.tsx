import { useId } from "react";
import { sourceCard } from "../../../domain/board/actionItems";
import { actionItemActions, isArchived, NO_ACTION_ITEM_ACTIONS } from "../../../domain/board/capabilities";
import { useBoardMembership } from "../../../hooks/board/useBoardMembership";
import type { CardFocus } from "../../../hooks/board/useCardFocus";
import type { BoardSnapshot } from "../../../types/api";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { ActionItemRow } from "./ActionItemRow";
import styles from "./ActionItems.module.css";

interface ActionItemsPanelProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cardFocus: CardFocus;
}

export function ActionItemsPanel({ snapshot, writable, cardFocus }: ActionItemsPanelProps) {
    const headingId = useId();
    const { board, board_id: boardId, action_items: items } = snapshot;
    const shown = board.stage === "actions" || items.length > 0;
    const membership = useBoardMembership(boardId, shown && writable && !isArchived(board));

    const actions = writable ? actionItemActions(snapshot, membership.member) : NO_ACTION_ITEM_ACTIONS;
    const people = actions.name ? membership.people : null;

    if (!shown) {
        return null;
    }

    return (
        <section aria-labelledby={headingId} className={styles.panel}>
            <div className={styles.header}>
                <h2 id={headingId} className={styles.heading}>
                    Action items <span className={styles.count}>{items.length}</span>
                </h2>
                {board.stage === "actions" && actions.add && (
                    <p className={styles.description}>Actions are added from a card&apos;s + Action button.</p>
                )}
            </div>
            {membership.failure !== null && (
                <LoadFailure message={membership.failure.message} onRetry={membership.retry} />
            )}
            {items.length === 0 ? (
                <p className={styles.empty}>No action items yet.</p>
            ) : (
                <ul className={styles.list}>
                    {items.map(item => (
                        <ActionItemRow
                            key={item.id}
                            boardId={boardId}
                            item={item}
                            source={sourceCard(snapshot, item)}
                            actions={actions}
                            people={people}
                            cardFocus={cardFocus}
                        />
                    ))}
                </ul>
            )}
        </section>
    );
}
