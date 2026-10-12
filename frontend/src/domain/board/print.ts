import type { BoardSnapshot, CardView, ColumnView } from "../../types/api";
import { reactionOption } from "./reactions";
import { hiddenCount } from "./selectors";
import { compareByPosition } from "./sort";
import { type CardStack, cardStacks, stackCards } from "./stacks";
import { stackVotes } from "./votes";

export type PrintAction = "print" | "png";

export interface PrintReaction {
    key: string;
    emoji: string;
    label: string;
    count: number;
}

export interface PrintCard {
    id: string;
    body: string;
    votes: number | null;
    pinned: boolean;
    imagePath: string | null;
    reactions: PrintReaction[];
    merged: PrintMergedCard[];
}

export interface PrintMergedCard {
    id: string;
    body: string;
}

export interface PrintColumn {
    column: ColumnView;
    cards: PrintCard[];
    hidden: number;
}

export const BOARD_PRINT_ROUTE = "/boards/:boardId/print";

export const PRINT_ACTION_PARAM = "then";

const PRINT_ACTIONS: readonly PrintAction[] = ["print", "png"];

const UNSAFE_FILE_CHARACTERS = /[/\\:*?"<>|]/g;

const CONTROL_CHARACTERS = /\p{Cc}/gu;

const RUNS_OF_SPACE = /\s+/g;

function isPrintAction(value: string | null): value is PrintAction {
    return PRINT_ACTIONS.some(action => action === value);
}

export function boardPrintPath(boardId: string, then: PrintAction): string {
    const query = new URLSearchParams({ [PRINT_ACTION_PARAM]: then });

    return `/boards/${encodeURIComponent(boardId)}/print?${query.toString()}`;
}

export function printAction(search: string): PrintAction | null {
    const then = new URLSearchParams(search).get(PRINT_ACTION_PARAM);

    return isPrintAction(then) ? then : null;
}

function twoDigits(value: number): string {
    return String(value).padStart(2, "0");
}

export function boardImageFileName(title: string, date: Date): string {
    const name = title
        .replace(CONTROL_CHARACTERS, " ")
        .replace(UNSAFE_FILE_CHARACTERS, "-")
        .replace(RUNS_OF_SPACE, " ")
        .trim();
    const day = `${date.getUTCFullYear()}-${twoDigits(date.getUTCMonth() + 1)}-${twoDigits(date.getUTCDate())}`;

    return `RetroTide - ${name} - ${day}.png`;
}

function printReactions(card: CardView): PrintReaction[] {
    return (card.reactions ?? []).map(({ reaction, count }) => ({ ...reactionOption(reaction), count }));
}

function printCard(snapshot: BoardSnapshot, stack: CardStack): PrintCard {
    const { card } = stack;

    return {
        id: card.id,
        body: card.body,
        votes: stackVotes(stackCards(stack), snapshot.me).total,
        pinned: card.pinned === true,
        imagePath: card.image_path ?? null,
        reactions: printReactions(card),
        merged: stack.merged.map(merged => ({ id: merged.id, body: merged.body })),
    };
}

function comparePinnedFirst(left: CardStack, right: CardStack): number {
    const pinned = Number(right.card.pinned === true) - Number(left.card.pinned === true);

    if (pinned !== 0) {
        return pinned;
    }

    return compareByPosition(left.card, right.card);
}

export function printColumns(snapshot: BoardSnapshot): PrintColumn[] {
    return snapshot.columns.toSorted(compareByPosition).map(column => {
        const stacks = cardStacks(snapshot.cards.filter(card => card.column_id === column.id));

        return {
            column,
            cards: stacks.toSorted(comparePinnedFirst).map(stack => printCard(snapshot, stack)),
            hidden: hiddenCount(snapshot.hidden_counts, column.id),
        };
    });
}
