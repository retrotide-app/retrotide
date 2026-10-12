import type { AuthorView, BoardSnapshot, BoardView, MeView, SortMode } from "../../types/api";
import { compareBytes } from "./sort";
import { type CardStack, stackCards } from "./stacks";
import { stackVotes } from "./votes";

export type CardSort = "manual" | "votes" | "date" | "random";

export interface CardFilter {
    sort: CardSort;
    query: string;
    authorId: string | null;
    seed: number;
}

export interface CardSortChoice {
    value: CardSort;
    label: string;
}

export const UNFILTERED: CardFilter = { sort: "manual", query: "", authorId: null, seed: 0 };

const SORT_LABELS: Readonly<Record<CardSort, string>> = {
    manual: "Board order",
    votes: "Most votes",
    date: "Newest first",
    random: "Shuffled",
};

const NAMED_SORTS: readonly CardSort[] = ["manual", "votes", "date", "random"];

const ANONYMOUS_SORTS: readonly CardSort[] = ["manual", "votes", "random"];

const FNV_OFFSET = 2166136261;

const FNV_PRIME = 16777619;

function namesAuthors(board: BoardView): boolean {
    return board.anonymity === "named";
}

function shuffleKey(seed: number, id: string): number {
    let hash = FNV_OFFSET ^ seed;

    for (const character of id) {
        hash = Math.imul(hash ^ (character.codePointAt(0) ?? 0), FNV_PRIME);
    }

    return hash >>> 0;
}

function normalised(text: string): string {
    return text.trim().toLocaleLowerCase();
}

export function sortChoices(board: BoardView): CardSortChoice[] {
    const sorts = namesAuthors(board) ? NAMED_SORTS : ANONYMOUS_SORTS;

    return sorts.map(value => ({ value, label: SORT_LABELS[value] }));
}

export function cardSortFrom(value: string, board: BoardView): CardSort | undefined {
    return sortChoices(board).find(choice => choice.value === value)?.value;
}

export function sortInForce(chosen: CardSort | null, boardSort: SortMode, enforced: boolean): CardSort {
    if (enforced || chosen === null) {
        return boardSort;
    }

    return chosen;
}

export function shownSort(filter: CardFilter, board: BoardView): CardSort {
    if (filter.sort === "date" && !namesAuthors(board)) {
        return "manual";
    }

    return filter.sort;
}

export function authorFilterOf(filter: CardFilter, board: BoardView): string | null {
    return namesAuthors(board) ? filter.authorId : null;
}

export function isFiltering(filter: CardFilter, board: BoardView): boolean {
    return normalised(filter.query) !== "" || authorFilterOf(filter, board) !== null;
}

export function keepsBoardOrder(filter: CardFilter, board: BoardView): boolean {
    return shownSort(filter, board) === "manual" && !isFiltering(filter, board);
}

export function authorChoices(snapshot: BoardSnapshot): AuthorView[] {
    if (!namesAuthors(snapshot.board)) {
        return [];
    }

    const authors = new Map<string, AuthorView>();

    for (const card of snapshot.cards) {
        if (card.author !== null && !authors.has(card.author.participant_id)) {
            authors.set(card.author.participant_id, card.author);
        }
    }

    return [...authors.values()].toSorted((a, b) => a.display_name.localeCompare(b.display_name));
}

export function stackMatches(stack: CardStack, filter: CardFilter, board: BoardView): boolean {
    const cards = stackCards(stack);
    const query = normalised(filter.query);
    const authorId = authorFilterOf(filter, board);

    if (query !== "" && !cards.some(card => card.body.toLocaleLowerCase().includes(query))) {
        return false;
    }

    if (authorId !== null && !cards.some(card => card.author?.participant_id === authorId)) {
        return false;
    }

    return true;
}

function stackTotal(stack: CardStack, me: MeView): number {
    return stackVotes(stackCards(stack), me).total ?? 0;
}

export function sortStacks(
    stacks: readonly CardStack[],
    filter: CardFilter,
    board: BoardView,
    me: MeView,
): CardStack[] {
    const ordered = orderedStacks(stacks, filter, board, me);

    return [
        ...ordered.filter(stack => stack.card.pinned === true),
        ...ordered.filter(stack => stack.card.pinned !== true),
    ];
}

function orderedStacks(stacks: readonly CardStack[], filter: CardFilter, board: BoardView, me: MeView): CardStack[] {
    const order = new Map(stacks.map((stack, index) => [stack.card.id, index]));
    const byOrder = (a: CardStack, b: CardStack) => (order.get(a.card.id) ?? 0) - (order.get(b.card.id) ?? 0);

    switch (shownSort(filter, board)) {
        case "manual":
            return [...stacks];
        case "votes":
            return stacks.toSorted((a, b) => {
                const difference = stackTotal(b, me) - stackTotal(a, me);

                return difference !== 0 ? difference : byOrder(a, b);
            });
        case "date":
            return stacks.toSorted((a, b) => {
                const difference = compareBytes(b.card.created_at ?? "", a.card.created_at ?? "");

                return difference !== 0 ? difference : byOrder(a, b);
            });
        case "random":
            return stacks.toSorted((a, b) => {
                const difference = shuffleKey(filter.seed, a.card.id) - shuffleKey(filter.seed, b.card.id);

                return difference !== 0 ? difference : byOrder(a, b);
            });
    }
}
