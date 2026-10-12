import type { BoardSnapshot } from "../../types/api";
import { ellipsise } from "../../utils/text";
import {
    type CardDrop,
    cardDropMoves,
    cardDropTarget,
    type ColumnDrop,
    columnDropMoves,
    columnDropTarget,
} from "./dropTarget";
import { cardsInColumn } from "./selectors";
import { compareByPosition } from "./sort";
import { stackTopId } from "./stacks";

export type BoardDragKind = "card" | "card-merge" | "column" | "column-cards";

export type BoardDragData = {
    kind: BoardDragKind;
    id: string;
};

export type BoardMove =
    | { kind: "card"; cardId: string; drop: CardDrop }
    | { kind: "merge"; cardId: string; targetCardId: string }
    | { kind: "column"; columnId: string; drop: ColumnDrop };

export interface DropRules {
    reorder: boolean;
    merge: boolean;
}

export type DropSide = "before" | "after";

export type DropPreview =
    | { kind: "merge"; cardId: string }
    | { kind: "line"; cardId: string; side: DropSide }
    | { kind: "column-end"; columnId: string };

export interface MergeChoice {
    cardId: string;
    label: string;
}

const DEFAULT_DROP_RULES: DropRules = { reorder: true, merge: false };

const MERGE_LABEL_CHARS = 80;

const BOARD_DRAG_KINDS: ReadonlySet<unknown> = new Set<BoardDragKind>(["card", "card-merge", "column", "column-cards"]);

export interface CardMoveChoice {
    key: string;
    label: string;
    drop: CardDrop;
}

const END_SLOT = Number.POSITIVE_INFINITY;

function cardSideDrop(snapshot: BoardSnapshot, cardId: string, anchorId: string, side: DropSide): CardDrop | null {
    const anchor = snapshot.cards.find(candidate => candidate.id === anchorId);

    if (anchor === undefined || anchor.id === cardId) {
        return null;
    }

    const others = cardsInColumn(snapshot.cards, anchor.column_id).filter(candidate => candidate.id !== cardId);
    const index = others.findIndex(candidate => candidate.id === anchor.id);

    if (index === -1) {
        return null;
    }

    if (side === "after") {
        return { column_id: anchor.column_id, after_card_id: anchor.id };
    }

    return { column_id: anchor.column_id, after_card_id: index === 0 ? null : others[index - 1].id };
}

function resolveCardDrop(
    snapshot: BoardSnapshot,
    cardId: string,
    onto: BoardDragData,
    side: DropSide,
): BoardMove | null {
    const card = snapshot.cards.find(candidate => candidate.id === cardId);

    if (card === undefined) {
        return null;
    }

    const drop =
        onto.kind === "card"
            ? cardSideDrop(snapshot, card.id, onto.id, side)
            : cardDropTarget(cardsInColumn(snapshot.cards, onto.id), card.id, onto.id, END_SLOT);

    if (drop === null || !snapshot.columns.some(column => column.id === drop.column_id)) {
        return null;
    }

    if (!cardDropMoves(cardsInColumn(snapshot.cards, card.column_id), card, drop)) {
        return null;
    }

    return { kind: "card", cardId: card.id, drop };
}

function resolveColumnDrop(snapshot: BoardSnapshot, columnId: string, onto: BoardDragData): BoardMove | null {
    if (onto.kind !== "column") {
        return null;
    }

    const columns = snapshot.columns.toSorted(compareByPosition);
    const slot = columns.findIndex(column => column.id === onto.id);

    if (slot === -1 || !columns.some(column => column.id === columnId)) {
        return null;
    }

    const drop = columnDropTarget(columns, columnId, slot);

    if (!columnDropMoves(columns, columnId, drop)) {
        return null;
    }

    return { kind: "column", columnId, drop };
}

export function cardMoveChoices(snapshot: BoardSnapshot, cardId: string): CardMoveChoice[] {
    const card = snapshot.cards.find(candidate => candidate.id === cardId);

    if (card === undefined) {
        return [];
    }

    const ownCards = cardsInColumn(snapshot.cards, card.column_id);
    const index = ownCards.findIndex(candidate => candidate.id === card.id);
    const wanted: { label: string; columnId: string; slot: number }[] = [
        { label: "Up one place", columnId: card.column_id, slot: index - 1 },
        { label: "Down one place", columnId: card.column_id, slot: index + 1 },
    ];

    for (const column of snapshot.columns.toSorted(compareByPosition)) {
        wanted.push(
            { label: `Top of ${column.title}`, columnId: column.id, slot: 0 },
            { label: `Bottom of ${column.title}`, columnId: column.id, slot: END_SLOT },
        );
    }

    const choices: CardMoveChoice[] = [];

    for (const { label, columnId, slot } of wanted) {
        if (slot < 0) {
            continue;
        }

        const drop = cardDropTarget(cardsInColumn(snapshot.cards, columnId), card.id, columnId, slot);
        const key = `${drop.column_id}/${drop.after_card_id ?? ""}`;

        if (!cardDropMoves(ownCards, card, drop) || choices.some(choice => choice.key === key)) {
            continue;
        }

        choices.push({ key, label, drop });
    }

    return choices;
}

function resolveMerge(snapshot: BoardSnapshot, cardId: string, targetId: string): BoardMove | null {
    const card = snapshot.cards.find(candidate => candidate.id === cardId);
    const targetTop = stackTopId(snapshot.cards, targetId);

    if (card === undefined || targetTop === null || targetTop === stackTopId(snapshot.cards, cardId)) {
        return null;
    }

    return { kind: "merge", cardId, targetCardId: targetTop };
}

export function mergeChoices(snapshot: BoardSnapshot, cardId: string): MergeChoice[] {
    const ownTop = stackTopId(snapshot.cards, cardId);

    if (ownTop === null) {
        return [];
    }

    const columns = snapshot.columns.toSorted(compareByPosition);

    return columns.flatMap(column =>
        cardsInColumn(snapshot.cards, column.id)
            .filter(card => card.id !== ownTop)
            .map(card => ({ cardId: card.id, label: `${column.title}: ${ellipsise(card.body, MERGE_LABEL_CHARS)}` })),
    );
}

export function dropPreview(snapshot: BoardSnapshot, move: BoardMove | null): DropPreview | null {
    if (move === null || move.kind === "column") {
        return null;
    }

    if (move.kind === "merge") {
        return { kind: "merge", cardId: move.targetCardId };
    }

    const { column_id: columnId, after_card_id: afterCardId } = move.drop;

    if (afterCardId !== null) {
        return { kind: "line", cardId: afterCardId, side: "after" };
    }

    const first = cardsInColumn(snapshot.cards, columnId).find(candidate => candidate.id !== move.cardId);

    return first === undefined ? { kind: "column-end", columnId } : { kind: "line", cardId: first.id, side: "before" };
}

export function sameDropPreview(a: DropPreview | null, b: DropPreview | null): boolean {
    return JSON.stringify(a) === JSON.stringify(b);
}

export function readDragData(data: unknown): BoardDragData | null {
    if (typeof data !== "object" || data === null || !("kind" in data) || !("id" in data)) {
        return null;
    }

    const { kind, id } = data;

    if (!BOARD_DRAG_KINDS.has(kind) || typeof id !== "string") {
        return null;
    }

    return { kind: kind as BoardDragKind, id };
}

export function resolveDrop(
    snapshot: BoardSnapshot,
    dragged: BoardDragData,
    onto: BoardDragData,
    rules: DropRules = DEFAULT_DROP_RULES,
    side: DropSide = "before",
): BoardMove | null {
    if (dragged.kind === "card" && onto.kind === "card-merge") {
        return rules.merge ? resolveMerge(snapshot, dragged.id, onto.id) : null;
    }

    if (dragged.kind === "card") {
        return rules.reorder ? resolveCardDrop(snapshot, dragged.id, onto, side) : null;
    }

    if (dragged.kind === "column") {
        return resolveColumnDrop(snapshot, dragged.id, onto);
    }

    return null;
}
