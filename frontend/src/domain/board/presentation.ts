import type { ActionItemView, BoardSnapshot, CardView, CommentView } from "../../types/api";
import { columnCardsHidden } from "./capabilities";
import { type CardFilter, sortStacks, UNFILTERED } from "./cardFilter";
import { commentsOnStack } from "./comments";
import { compareByPosition } from "./sort";
import { cardStacks, stackCards, stackOf } from "./stacks";

export interface PresentedStack {
    card: CardView;
    merged: CardView[];
    comments: CommentView[];
    actionItems: ActionItemView[];
}

export interface PresentationProgress {
    position: number;
    total: number;
}

export function presentationOrder(snapshot: BoardSnapshot): string[] {
    const boardSort: CardFilter = { ...UNFILTERED, sort: snapshot.settings.sort_mode };

    return snapshot.columns
        .toSorted(compareByPosition)
        .filter(column => !column.hidden && !columnCardsHidden(snapshot, column.id))
        .flatMap(column => {
            const stacks = cardStacks(snapshot.cards.filter(card => card.column_id === column.id));
            const shown = stacks.filter(stack => !stack.card.hidden);

            return sortStacks(shown, boardSort, snapshot.board, snapshot.me).map(stack => stack.card.id);
        });
}

export function nextPresented(order: readonly string[], current: string): string | null {
    const index = order.indexOf(current);

    if (index === -1) {
        return null;
    }

    return order.at(index + 1) ?? null;
}

export function previousPresented(order: readonly string[], current: string): string | null {
    const index = order.indexOf(current);

    if (index <= 0) {
        return null;
    }

    return order[index - 1];
}

export function presentationProgress(order: readonly string[], current: string): PresentationProgress | null {
    const index = order.indexOf(current);

    if (index === -1) {
        return null;
    }

    return { position: index + 1, total: order.length };
}

export function presentedStack(snapshot: BoardSnapshot, cardId: string): PresentedStack | null {
    const stack = stackOf(snapshot.cards, cardId);

    if (stack === null) {
        return null;
    }

    const cards = stackCards(stack);
    const ids = new Set(cards.map(card => card.id));

    return {
        card: stack.card,
        merged: stack.merged,
        comments: commentsOnStack(snapshot.comments, cards),
        actionItems: snapshot.action_items.filter(item => item.source_card_id !== null && ids.has(item.source_card_id)),
    };
}
