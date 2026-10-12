import type { CardView, ColumnView } from "../../types/api";
import { compareByPosition } from "./sort";

export interface CardDrop {
    column_id: string;
    after_card_id: string | null;
}

export interface ColumnDrop {
    after_column_id: string | null;
}

interface Placed {
    id: string;
    position: string;
}

function anchorAt(items: readonly Placed[], movedId: string, slot: number): string | null {
    const others = items.filter(item => item.id !== movedId).toSorted(compareByPosition);
    const clamped = Math.min(Math.max(Math.trunc(slot), 0), others.length);

    if (clamped === 0) {
        return null;
    }

    return others[clamped - 1].id;
}

function predecessorOf(items: readonly Placed[], id: string): string | null | undefined {
    const sorted = items.toSorted(compareByPosition);
    const index = sorted.findIndex(item => item.id === id);

    if (index === -1) {
        return undefined;
    }

    return index === 0 ? null : sorted[index - 1].id;
}

export function cardDropTarget(
    columnCards: readonly CardView[],
    cardId: string,
    columnId: string,
    slot: number,
): CardDrop {
    return { column_id: columnId, after_card_id: anchorAt(columnCards, cardId, slot) };
}

export function columnDropTarget(columns: readonly ColumnView[], columnId: string, slot: number): ColumnDrop {
    return { after_column_id: anchorAt(columns, columnId, slot) };
}

export function cardDropMoves(columnCards: readonly CardView[], card: CardView, drop: CardDrop): boolean {
    if (drop.column_id !== card.column_id) {
        return true;
    }

    return predecessorOf(columnCards, card.id) !== drop.after_card_id;
}

export function columnDropMoves(columns: readonly ColumnView[], columnId: string, drop: ColumnDrop): boolean {
    return predecessorOf(columns, columnId) !== drop.after_column_id;
}
