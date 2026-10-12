import type { CardView } from "../../types/api";
import { compareByPosition } from "./sort";

export interface CardStack {
    card: CardView;
    merged: CardView[];
}

function heldIds(cards: readonly CardView[]): Set<string> {
    return new Set(cards.map(card => card.id));
}

function isTopIn(card: CardView, held: ReadonlySet<string>): boolean {
    return card.parent_card_id === null || !held.has(card.parent_card_id);
}

export function topCards(cards: readonly CardView[]): CardView[] {
    const held = heldIds(cards);

    return cards.filter(card => isTopIn(card, held));
}

export function cardStacks(cards: readonly CardView[]): CardStack[] {
    const held = heldIds(cards);
    const merged = new Map<string, CardView[]>();
    const tops: CardView[] = [];

    for (const card of cards.toSorted(compareByPosition)) {
        const parentId = card.parent_card_id;

        if (parentId === null || !held.has(parentId)) {
            tops.push(card);
            continue;
        }

        const under = merged.get(parentId) ?? [];
        under.push(card);
        merged.set(parentId, under);
    }

    return tops.map(card => ({ card, merged: merged.get(card.id) ?? [] }));
}

export function stackCards(stack: CardStack): CardView[] {
    return [stack.card, ...stack.merged];
}

export function stackTopId(cards: readonly CardView[], cardId: string): string | null {
    const card = cards.find(candidate => candidate.id === cardId);

    if (card === undefined) {
        return null;
    }

    return isTopIn(card, heldIds(cards)) ? card.id : card.parent_card_id;
}

export function stackOf(cards: readonly CardView[], cardId: string): CardStack | null {
    const topId = stackTopId(cards, cardId);

    if (topId === null) {
        return null;
    }

    return cardStacks(cards).find(stack => stack.card.id === topId) ?? null;
}
