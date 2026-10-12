import type { BoardSettingsView, BoardSnapshot, CardView, MeView } from "../../types/api";

export interface StackVotes {
    total: number | null;
    mine: number;
}

function sumOf(counts: Readonly<Record<string, number>>): number {
    return Object.values(counts).reduce((sum, count) => sum + count, 0);
}

export function meVersion(snapshot: BoardSnapshot): number {
    return snapshot.me.version ?? snapshot.version;
}

export function stampMe(snapshot: BoardSnapshot): BoardSnapshot {
    if (snapshot.me.version !== undefined) {
        return snapshot;
    }

    return { ...snapshot, me: { ...snapshot.me, version: snapshot.version } };
}

export function withMe(snapshot: BoardSnapshot, me: MeView, version: number): BoardSnapshot {
    if (version <= meVersion(snapshot)) {
        return snapshot;
    }

    return { ...snapshot, me: { votes_remaining: me.votes_remaining, my_votes: { ...me.my_votes }, version } };
}

export function spentVotes(me: MeView, maxVotes: number): number {
    return Math.max(sumOf(me.my_votes), maxVotes - me.votes_remaining);
}

export function meAfterSettings(me: MeView, previous: BoardSettingsView, next: BoardSettingsView): MeView {
    const spent = spentVotes(me, previous.max_votes);

    return { ...me, votes_remaining: Math.max(0, next.max_votes - spent) };
}

export function meAfterReset(me: MeView, settings: BoardSettingsView): MeView {
    return { ...me, votes_remaining: settings.max_votes, my_votes: {} };
}

export function meWithoutCards(me: MeView, settings: BoardSettingsView, cardIds: ReadonlySet<string>): MeView {
    const kept: Record<string, number> = {};
    let refunded = 0;

    for (const [cardId, count] of Object.entries(me.my_votes)) {
        if (cardIds.has(cardId)) {
            refunded += count;
            continue;
        }

        kept[cardId] = count;
    }

    if (refunded === 0) {
        return me;
    }

    const spent = spentVotes(me, settings.max_votes) - refunded;

    return { ...me, votes_remaining: Math.max(0, settings.max_votes - spent), my_votes: kept };
}

export function meWithoutVotesOn(me: MeView, cardIds: ReadonlySet<string>): MeView {
    const kept: Record<string, number> = {};
    let dropped = false;

    for (const [cardId, count] of Object.entries(me.my_votes)) {
        if (cardIds.has(cardId)) {
            dropped = true;
            continue;
        }

        kept[cardId] = count;
    }

    if (!dropped) {
        return me;
    }

    return { ...me, my_votes: kept };
}

export function stackVotes(stack: readonly CardView[], me: MeView): StackVotes {
    let total: number | null = 0;
    let mine = 0;

    for (const card of stack) {
        mine += me.my_votes[card.id] ?? 0;

        if (total === null || card.votes.total === null) {
            total = null;
            continue;
        }

        total += card.votes.total;
    }

    return { total, mine };
}

export function heldVoteCard(stack: readonly CardView[], me: MeView): string | null {
    const holding = stack.find(card => (me.my_votes[card.id] ?? 0) > 0);

    return holding?.id ?? null;
}

export const COLUMN_VOTE_LIMIT_MAX = 99;

const WHOLE_NUMBER = /^\d+$/;

export function columnVoteLimitFrom(text: string): number | null {
    const trimmed = text.trim();

    if (!WHOLE_NUMBER.test(trimmed)) {
        return null;
    }

    const limit = Number(trimmed);

    return limit >= 1 && limit <= COLUMN_VOTE_LIMIT_MAX ? limit : null;
}

export function columnVotesUsed(snapshot: BoardSnapshot, columnId: string): number {
    let used = 0;

    for (const card of snapshot.cards) {
        if (card.column_id === columnId) {
            used += snapshot.me.my_votes[card.id] ?? 0;
        }
    }

    return used;
}

export function columnVotesLeft(snapshot: BoardSnapshot, columnId: string): number | null {
    const column = snapshot.columns.find(held => held.id === columnId);

    if (column === undefined || column.max_votes === null) {
        return null;
    }

    return Math.max(0, column.max_votes - columnVotesUsed(snapshot, columnId));
}

export function votesLeftOnCard(snapshot: BoardSnapshot, card: CardView): number {
    const left = snapshot.me.votes_remaining;
    const cap = snapshot.settings.max_votes_per_card;

    if (cap === 0) {
        return left;
    }

    return Math.max(0, Math.min(left, cap - (snapshot.me.my_votes[card.id] ?? 0)));
}
