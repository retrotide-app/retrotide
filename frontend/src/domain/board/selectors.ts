import type { BoardSnapshot, CardView, ColumnView } from "../../types/api";
import { type CardFilter, isFiltering, sortStacks, stackMatches, UNFILTERED } from "./cardFilter";
import { compareByPosition } from "./sort";
import { type CardStack, cardStacks, topCards } from "./stacks";

export interface HiddenPlaceholder {
    key: string;
}

export interface BoardColumnModel {
    column: ColumnView;
    cards: CardView[];
    stacks: CardStack[];
    placeholders: HiddenPlaceholder[];
    total: number;
    filtered: boolean;
}

export function cardsInColumn(cards: readonly CardView[], columnId: string): CardView[] {
    return topCards(cards.filter(card => card.column_id === columnId)).toSorted(compareByPosition);
}

export function hiddenCount(hiddenCounts: Readonly<Record<string, number>>, columnId: string): number {
    return hiddenCounts[columnId] ?? 0;
}

export function hiddenPlaceholders(
    hiddenCounts: Readonly<Record<string, number>>,
    columnId: string,
): HiddenPlaceholder[] {
    const count = hiddenCount(hiddenCounts, columnId);

    return Array.from({ length: count }, (_, index) => ({ key: `${columnId}:hidden:${index}` }));
}

export function boardItemKeys(snapshot: BoardSnapshot): string[] {
    return boardColumns(snapshot).flatMap(model => [
        ...model.stacks.flatMap(stack => [stack.card.id, ...stack.merged.map(card => card.id)]),
        ...model.placeholders.map(placeholder => placeholder.key),
    ]);
}

export function boardColumns(snapshot: BoardSnapshot, filter: CardFilter = UNFILTERED): BoardColumnModel[] {
    const filtering = isFiltering(filter, snapshot.board);

    return snapshot.columns.toSorted(compareByPosition).map(column => {
        const inColumn = cardStacks(snapshot.cards.filter(card => card.column_id === column.id));
        const shown = inColumn.filter(stack => stackMatches(stack, filter, snapshot.board));
        const stacks = sortStacks(shown, filter, snapshot.board, snapshot.me);
        const placeholders = filtering ? [] : hiddenPlaceholders(snapshot.hidden_counts, column.id);
        const cardCount = stacks.reduce((sum, stack) => sum + 1 + stack.merged.length, 0);

        return {
            column,
            cards: stacks.map(stack => stack.card),
            stacks,
            placeholders,
            total: cardCount + placeholders.length,
            filtered: filtering,
        };
    });
}
