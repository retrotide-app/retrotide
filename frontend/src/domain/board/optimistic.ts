import type { BoardSnapshot } from "../../types/api";
import type { CardDrop, ColumnDrop } from "./dropTarget";
import { compareByPosition } from "./sort";
import { topCards } from "./stacks";
import { meVersion } from "./votes";

interface Placed {
    id: string;
    position: string;
}

export type VoteDelta = 1 | -1;

export type PendingChange =
    | { kind: "card-move"; cardId: string; drop: CardDrop }
    | { kind: "column-move"; columnId: string; drop: ColumnDrop }
    | { kind: "vote"; cardId: string; delta: VoteDelta };

export interface PendingCommand {
    change: PendingChange;
    answeredAt: number | null;
}

const LOCAL_KEY_WIDTH = 6;

function placedAfter<T extends Placed>(items: readonly T[], moved: T, afterId: string | null): T[] {
    const others = items.filter(item => item.id !== moved.id).toSorted(compareByPosition);
    const anchor = afterId === null ? -1 : others.findIndex(item => item.id === afterId);
    const slot = afterId !== null && anchor === -1 ? others.length : anchor + 1;
    const ordered = [...others.slice(0, slot), moved, ...others.slice(slot)];

    return ordered.map((item, index) => ({ ...item, position: String(index).padStart(LOCAL_KEY_WIDTH, "0") }));
}

export function withCardMoved(snapshot: BoardSnapshot, cardId: string, drop: CardDrop): BoardSnapshot {
    const card = snapshot.cards.find(candidate => candidate.id === cardId);

    if (card === undefined) {
        return snapshot;
    }

    const moved = { ...card, column_id: drop.column_id, parent_card_id: null };
    const target = topCards(snapshot.cards).filter(candidate => candidate.column_id === drop.column_id);
    const placed = new Map(placedAfter(target, moved, drop.after_card_id).map(item => [item.id, item]));

    return {
        ...snapshot,
        cards: snapshot.cards.map(candidate => {
            const next = placed.get(candidate.id);

            if (next !== undefined) {
                return next;
            }

            return candidate.parent_card_id === cardId ? { ...candidate, column_id: drop.column_id } : candidate;
        }),
    };
}

export function withColumnMoved(snapshot: BoardSnapshot, columnId: string, drop: ColumnDrop): BoardSnapshot {
    const column = snapshot.columns.find(candidate => candidate.id === columnId);

    if (column === undefined) {
        return snapshot;
    }

    return { ...snapshot, columns: placedAfter(snapshot.columns, column, drop.after_column_id) };
}

function withMyVote(snapshot: BoardSnapshot, cardId: string, delta: VoteDelta): BoardSnapshot {
    const { me } = snapshot;
    const count = Math.max(0, (me.my_votes[cardId] ?? 0) + delta);
    const { [cardId]: _previous, ...others } = me.my_votes;
    const myVotes = count === 0 ? others : { ...others, [cardId]: count };

    return {
        ...snapshot,
        me: { ...me, votes_remaining: Math.max(0, me.votes_remaining - delta), my_votes: myVotes },
    };
}

function withTotal(snapshot: BoardSnapshot, cardId: string, delta: VoteDelta): BoardSnapshot {
    return {
        ...snapshot,
        cards: snapshot.cards.map(card => {
            if (card.id !== cardId || card.votes.total === null) {
                return card;
            }

            return { ...card, votes: { total: Math.max(0, card.votes.total + delta) } };
        }),
    };
}

export function withVote(
    snapshot: BoardSnapshot,
    cardId: string,
    delta: VoteDelta,
    answeredAt: number | null,
): BoardSnapshot {
    if (!snapshot.cards.some(card => card.id === cardId)) {
        return snapshot;
    }

    const counted =
        answeredAt === null || snapshot.version < answeredAt ? withTotal(snapshot, cardId, delta) : snapshot;

    if (answeredAt !== null && meVersion(snapshot) >= answeredAt) {
        return counted;
    }

    return withMyVote(counted, cardId, delta);
}

export function isStillPending(command: PendingCommand, confirmedVersion: number): boolean {
    return command.answeredAt === null || command.answeredAt > confirmedVersion;
}

function isStillShown(command: PendingCommand, confirmed: BoardSnapshot): boolean {
    if (command.change.kind === "vote" && command.answeredAt !== null) {
        return isStillPending(command, confirmed.version) || meVersion(confirmed) < command.answeredAt;
    }

    return isStillPending(command, confirmed.version);
}

function withChange(snapshot: BoardSnapshot, command: PendingCommand): BoardSnapshot {
    const { change } = command;

    switch (change.kind) {
        case "card-move":
            return withCardMoved(snapshot, change.cardId, change.drop);
        case "column-move":
            return withColumnMoved(snapshot, change.columnId, change.drop);
        case "vote":
            return withVote(snapshot, change.cardId, change.delta, command.answeredAt);
    }
}

export function withPendingCommands(
    confirmed: BoardSnapshot,
    commands: readonly (PendingCommand | null)[],
): BoardSnapshot {
    let shown = confirmed;

    for (const command of commands) {
        if (command === null || !isStillShown(command, confirmed)) {
            continue;
        }

        shown = withChange(shown, command);
    }

    return shown;
}
