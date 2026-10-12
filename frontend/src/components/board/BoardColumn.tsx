import { useDroppable } from "@dnd-kit/react";
import { useId, useState } from "react";
import { type BoardActions, columnCardsHidden } from "../../domain/board/capabilities";
import type { BoardDragData, DropPreview } from "../../domain/board/drop";
import type { BoardColumnModel } from "../../domain/board/selectors";
import { columnVotesLeft } from "../../domain/board/votes";
import type { CardFocus } from "../../hooks/board/useCardFocus";
import type { BoardSnapshot } from "../../types/api";
import { Button } from "../Button/Button";
import { AddCardForm } from "./AddCardForm";
import styles from "./BoardColumn.module.css";
import colours from "./boardColours.module.css";
import { ColumnMenu } from "./ColumnMenu";
import { HiddenCards } from "./HiddenCards";
import { HiddenColumnNotice } from "./HiddenColumnNotice";
import { SortableCard } from "./SortableCard";

interface BoardColumnProps {
    snapshot: BoardSnapshot;
    model: BoardColumnModel;
    actions: BoardActions;
    writable: boolean;
    cardFocus: CardFocus;
    reorder?: boolean;
    merge?: boolean;
    preview?: DropPreview | null;
}

const LOW_COLLISION_PRIORITY = 1;

const COLUMN_CARDS_SUFFIX = ":cards";

function cardCount(total: number): string {
    return total === 1 ? "1 card" : `${total} cards`;
}

export function BoardColumn({
    snapshot,
    model,
    actions,
    writable,
    cardFocus,
    reorder = actions.moveCards,
    merge = false,
    preview = null,
}: BoardColumnProps) {
    const titleId = useId();
    const [showForMe, setShowForMe] = useState(false);
    const { column, stacks, placeholders, total } = model;
    const hidden = column.hidden;
    const frozen = hidden && !showForMe;
    const movable = reorder && !hidden;
    const endLine = !hidden && preview?.kind === "column-end" && preview.columnId === column.id;
    const shownPlaceholders = columnCardsHidden(snapshot, column.id) ? placeholders : [];
    const shownTotal = total - placeholders.length + shownPlaceholders.length;
    const votesLeft = snapshot.board.stage === "vote" ? columnVotesLeft(snapshot, column.id) : null;

    const cardsData: BoardDragData = { kind: "column-cards", id: column.id };
    const { ref: cardAreaRef } = useDroppable({
        id: `${column.id}${COLUMN_CARDS_SUFFIX}`,
        type: "column-cards",
        accept: "card",
        data: cardsData,
        collisionPriority: LOW_COLLISION_PRIORITY,
        disabled: !movable,
    });

    return (
        <section aria-labelledby={titleId} className={`${styles.column} ${colours[column.colour]}`}>
            <header className={styles.header}>
                <div className={styles.heading}>
                    <h2 id={titleId} className={styles.title} dir="auto">
                        {column.title}
                    </h2>
                    <span className={styles.count}>{cardCount(shownTotal)}</span>
                    <div className={styles.menu}>
                        <ColumnMenu snapshot={snapshot} column={column} actions={actions} />
                    </div>
                </div>
                {column.description !== "" && (
                    <p className={styles.description} dir="auto">
                        {column.description}
                    </p>
                )}
                {votesLeft !== null && column.max_votes !== null && (
                    <p className={styles.budget}>{`${votesLeft} of ${column.max_votes} votes left in this column`}</p>
                )}
            </header>
            {hidden && showForMe && (
                <div className={styles.hiddenBar}>
                    <span>Hidden from participants</span>
                    <Button type="button" size="small" variant="ghost" onClick={() => setShowForMe(false)}>
                        Hide again for me
                    </Button>
                </div>
            )}
            {actions.addCards && !hidden && <AddCardForm boardId={snapshot.board_id} column={column} />}
            {frozen ? (
                <HiddenColumnNotice
                    boardId={snapshot.board_id}
                    columnId={column.id}
                    onShowForMe={() => setShowForMe(true)}
                />
            ) : (
                <div ref={cardAreaRef} className={endLine ? `${styles.cards} ${styles.endLine}` : styles.cards}>
                    {stacks.length > 0 && (
                        <ol className={styles.cardList} aria-label={`Cards in ${column.title}`}>
                            {stacks.map((stack, cardIndex) => (
                                <SortableCard
                                    key={stack.card.id}
                                    snapshot={snapshot}
                                    stack={stack}
                                    index={cardIndex}
                                    writable={writable && !hidden}
                                    movable={movable}
                                    mergeable={merge && !hidden}
                                    preview={preview}
                                    cardFocus={cardFocus}
                                />
                            ))}
                        </ol>
                    )}
                    <HiddenCards placeholders={shownPlaceholders} />
                    {shownTotal === 0 && (
                        <p className={styles.empty}>{model.filtered ? "No matching cards." : "No cards yet."}</p>
                    )}
                </div>
            )}
        </section>
    );
}
