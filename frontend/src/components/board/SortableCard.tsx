import { pointerIntersection } from "@dnd-kit/collision";
import { useDroppable } from "@dnd-kit/react";
import { useSortable } from "@dnd-kit/react/sortable";
import { cardActions } from "../../domain/board/capabilities";
import type { BoardDragData, DropPreview } from "../../domain/board/drop";
import type { CardStack } from "../../domain/board/stacks";
import type { CardFocus } from "../../hooks/board/useCardFocus";
import type { BoardSnapshot } from "../../types/api";
import { BoardCard } from "./BoardCard";
import styles from "./BoardCard.module.css";

interface SortableCardProps {
    snapshot: BoardSnapshot;
    stack: CardStack;
    index: number;
    writable: boolean;
    movable: boolean;
    mergeable?: boolean;
    preview?: DropPreview | null;
    cardFocus: CardFocus;
}

const MERGE_SUFFIX = ":merge";

const MERGE_COLLISION_PRIORITY = 4;

function previewClass(preview: DropPreview | null, cardId: string): string {
    if (preview === null || preview.kind === "column-end" || preview.cardId !== cardId) {
        return "";
    }

    if (preview.kind === "merge") {
        return styles.mergeInto;
    }

    return preview.side === "before" ? styles.lineBefore : styles.lineAfter;
}

export function SortableCard({
    snapshot,
    stack,
    index,
    writable,
    movable,
    mergeable = false,
    preview = null,
    cardFocus,
}: SortableCardProps) {
    const { card, merged } = stack;
    const carries = movable && writable && cardActions(snapshot, card).move;
    const data: BoardDragData = { kind: "card", id: card.id };
    const { ref, isDragSource } = useSortable({
        id: card.id,
        index,
        group: card.column_id,
        type: "card",
        accept: "card",
        data,
        disabled: { draggable: !carries && !mergeable, droppable: !movable && !mergeable },
        plugins: [],
    });

    const mergeData: BoardDragData = { kind: "card-merge", id: card.id };
    const { ref: mergeRef } = useDroppable({
        id: `${card.id}${MERGE_SUFFIX}`,
        type: "card-merge",
        accept: "card",
        data: mergeData,
        collisionDetector: pointerIntersection,
        collisionPriority: MERGE_COLLISION_PRIORITY,
        disabled: !mergeable || isDragSource,
    });

    const draggable = carries || mergeable;
    const classes = [styles.slot, draggable ? styles.draggable : "", previewClass(preview, card.id)]
        .filter(Boolean)
        .join(" ");

    return (
        <li ref={ref} className={classes}>
            <BoardCard snapshot={snapshot} card={card} merged={merged} writable={writable} cardFocus={cardFocus} />
            <div ref={mergeRef} className={styles.mergeZone} aria-hidden="true" />
        </li>
    );
}
