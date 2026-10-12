import type {
    BoardEvent,
    BoardSnapshot,
    CardsRevealedEvent,
    CardView,
    ColumnRevealedEvent,
    ColumnView,
    MeView,
    PositionView,
    ReactionCount,
    ReactionKey,
    SpotlightView,
} from "../../types/api";
import { totalsHidden } from "./stages";
import { meAfterReset, meAfterSettings, meVersion, meWithoutCards, meWithoutVotesOn, stampMe } from "./votes";

interface Placed {
    id: string;
    position: string;
}

interface Owned {
    id: string;
    is_mine?: true;
}

function upsertById<T extends { id: string }>(items: readonly T[], next: T): T[] {
    if (!items.some(item => item.id === next.id)) {
        return [...items, next];
    }

    return items.map(item => (item.id === next.id ? next : item));
}

function withPositions<T extends Placed>(items: readonly T[], rewritten: readonly PositionView[]): T[] {
    if (rewritten.length === 0) {
        return [...items];
    }

    const keys = new Map(rewritten.map(placed => [placed.id, placed.position]));

    return items.map(item => {
        const position = keys.get(item.id);

        return position === undefined ? item : { ...item, position };
    });
}

function withoutCount(counts: Readonly<Record<string, number>>, columnId: string): Record<string, number> {
    const { [columnId]: _dropped, ...kept } = counts;

    return kept;
}

function keepOwnership<T extends Owned>(held: T | undefined, incoming: T): T {
    if (held?.is_mine !== true || incoming.is_mine === true) {
        return incoming;
    }

    return { ...incoming, is_mine: true };
}

function upsertOwned<T extends Owned>(items: readonly T[], incoming: T): T[] {
    const held = items.find(item => item.id === incoming.id);

    return upsertById(items, keepOwnership(held, incoming));
}

function keepHeldCard(held: CardView | undefined, incoming: CardView): CardView {
    const owned = keepOwnership(held, incoming);
    if (held === undefined) {
        return owned;
    }

    const reactions = owned.reactions ?? held.reactions;
    const myReactions = owned.my_reactions ?? held.my_reactions;
    if (reactions === owned.reactions && myReactions === owned.my_reactions) {
        return owned;
    }

    return withReactions(owned, reactions ?? [], myReactions ?? []);
}

function upsertCard(cards: readonly CardView[], incoming: CardView): CardView[] {
    const held = cards.find(card => card.id === incoming.id);

    return upsertById(cards, keepHeldCard(held, incoming));
}

function withReactions(card: CardView, reactions: readonly ReactionCount[], mine: readonly ReactionKey[]): CardView {
    const { reactions: _reactions, my_reactions: _mine, ...rest } = card;

    return {
        ...rest,
        ...(reactions.length > 0 ? { reactions: [...reactions] } : {}),
        ...(mine.length > 0 ? { my_reactions: [...mine] } : {}),
    };
}

function withPin(card: CardView, pinned: boolean): CardView {
    const { pinned: _pinned, ...rest } = card;

    return pinned ? { ...rest, pinned: true } : rest;
}

function withImage(card: CardView, imagePath: string | null): CardView {
    const { image_path: _imagePath, ...rest } = card;

    return imagePath === null ? rest : { ...rest, image_path: imagePath };
}

function mapCard(snapshot: BoardSnapshot, cardId: string, change: (card: CardView) => CardView): BoardSnapshot {
    return { ...snapshot, cards: snapshot.cards.map(card => (card.id === cardId ? change(card) : card)) };
}

function withBackground(snapshot: BoardSnapshot, backgroundPath: string | null): BoardSnapshot {
    const { background_path: _backgroundPath, ...board } = snapshot.board;

    return { ...snapshot, board: backgroundPath === null ? board : { ...board, background_path: backgroundPath } };
}

function withMeChange(snapshot: BoardSnapshot, version: number, change: (me: MeView) => MeView): BoardSnapshot {
    if (version <= meVersion(snapshot)) {
        return snapshot;
    }

    return { ...snapshot, me: { ...change(snapshot.me), version } };
}

function withoutCards(snapshot: BoardSnapshot, removed: ReadonlySet<string>, version: number): BoardSnapshot {
    const cards = snapshot.cards
        .filter(card => !removed.has(card.id))
        .map(card =>
            card.parent_card_id !== null && removed.has(card.parent_card_id) ? { ...card, parent_card_id: null } : card,
        );
    const comments = snapshot.comments.filter(comment => !removed.has(comment.card_id));
    const trimmed = { ...snapshot, cards, comments };

    return withMeChange(trimmed, version, me => meWithoutCards(me, snapshot.settings, removed));
}

function withoutRevealedColumns(columns: readonly ColumnView[]): ColumnView[] {
    return columns.map(column => (column.cards_revealed ? { ...column, cards_revealed: false } : column));
}

function withRevealedCards(snapshot: BoardSnapshot, event: CardsRevealedEvent): BoardSnapshot {
    const { stage, cards, comments } = event;
    const heldCards = new Map(snapshot.cards.map(card => [card.id, card]));
    const heldComments = new Map(snapshot.comments.map(comment => [comment.id, comment]));

    return {
        ...snapshot,
        board: { ...snapshot.board, cards_hidden: false, stage },
        columns: withoutRevealedColumns(snapshot.columns),
        cards: cards.map(card => keepHeldCard(heldCards.get(card.id), card)),
        comments: comments.map(comment => keepOwnership(heldComments.get(comment.id), comment)),
        hidden_counts: {},
    };
}

function withRehiddenCards(
    snapshot: BoardSnapshot,
    counts: Readonly<Record<string, number>>,
    version: number,
): BoardSnapshot {
    const mine = snapshot.cards.filter(card => card.is_mine === true);
    const kept = new Set(mine.map(card => card.id));
    const dropped = new Set(snapshot.cards.filter(card => !kept.has(card.id)).map(card => card.id));

    const hidden = {
        ...snapshot,
        board: { ...snapshot.board, cards_hidden: true },
        columns: withoutRevealedColumns(snapshot.columns),
        cards: mine.map(card => ({ ...card, hidden: true, parent_card_id: null })),
        comments: snapshot.comments.filter(comment => kept.has(comment.card_id)),
        hidden_counts: { ...counts },
    };

    return withMeChange(hidden, version, me => meWithoutVotesOn(me, dropped));
}

function cardIdsInColumn(cards: readonly CardView[], columnId: string): Set<string> {
    return new Set(cards.filter(card => card.column_id === columnId).map(card => card.id));
}

function spotlightWithout(
    spotlight: SpotlightView,
    cardIds: ReadonlySet<string>,
    columnId: string | null,
): SpotlightView {
    const presenting = spotlight.presenting_card_id;
    const highlighted = spotlight.highlighted_card_id;
    const focused = spotlight.focused_column_id;

    return {
        presenting_card_id: presenting !== null && cardIds.has(presenting) ? null : presenting,
        highlighted_card_id: highlighted !== null && cardIds.has(highlighted) ? null : highlighted,
        focused_column_id: columnId !== null && focused === columnId ? null : focused,
    };
}

function withRevealedColumn(snapshot: BoardSnapshot, event: ColumnRevealedEvent): BoardSnapshot {
    const heldCards = new Map(snapshot.cards.map(card => [card.id, card]));
    const heldComments = new Map(snapshot.comments.map(comment => [comment.id, comment]));
    const replaced = new Set([
        ...cardIdsInColumn(snapshot.cards, event.column_id),
        ...event.cards.map(card => card.id),
    ]);

    const cards = [
        ...snapshot.cards.filter(card => !replaced.has(card.id)),
        ...event.cards.map(card => keepHeldCard(heldCards.get(card.id), card)),
    ];
    const comments = [
        ...snapshot.comments.filter(comment => !replaced.has(comment.card_id)),
        ...event.comments.map(comment => keepOwnership(heldComments.get(comment.id), comment)),
    ];
    const columns = snapshot.columns.map(column =>
        column.id === event.column_id ? { ...column, cards_revealed: true } : column,
    );

    return {
        ...snapshot,
        columns,
        cards,
        comments,
        hidden_counts: withoutCount(snapshot.hidden_counts, event.column_id),
    };
}

function withHiddenColumn(snapshot: BoardSnapshot, columnId: string, version: number): BoardSnapshot {
    if (snapshot.viewer.capabilities.includes("board.facilitate")) {
        return {
            ...snapshot,
            columns: snapshot.columns.map(column => (column.id === columnId ? { ...column, hidden: true } : column)),
        };
    }

    const dropped = cardIdsInColumn(snapshot.cards, columnId);
    const hidden = {
        ...snapshot,
        columns: snapshot.columns.filter(column => column.id !== columnId),
        cards: snapshot.cards.filter(card => !dropped.has(card.id)),
        comments: snapshot.comments.filter(comment => !dropped.has(comment.card_id)),
        hidden_counts: withoutCount(snapshot.hidden_counts, columnId),
    };

    return withMeChange(hidden, version, me => meWithoutVotesOn(me, dropped));
}

function withColumnVotesReset(snapshot: BoardSnapshot, columnId: string, version: number): BoardSnapshot {
    const reset = cardIdsInColumn(snapshot.cards, columnId);
    const cards = snapshot.cards.map(card =>
        reset.has(card.id) ? { ...card, votes: { total: card.votes.total === null ? null : 0 } } : card,
    );

    return withMeChange({ ...snapshot, cards }, version, me => meWithoutCards(me, snapshot.settings, reset));
}

function placedCard(card: CardView, columnId: string, position: string, parentId: string | null): CardView {
    return { ...card, column_id: columnId, position, parent_card_id: parentId };
}

export function applyBoardEvent(snapshot: BoardSnapshot, event: BoardEvent): BoardSnapshot {
    switch (event.type) {
        case "resync":
            return snapshot;
        case "board_updated":
            return { ...snapshot, board: { ...snapshot.board, title: event.title, context: event.context } };
        case "settings_changed": {
            const cards = totalsHidden(event.settings, snapshot.board.stage)
                ? snapshot.cards.map(card => ({ ...card, votes: { total: null } }))
                : snapshot.cards;
            const next = { ...snapshot, cards, settings: { ...event.settings } };

            return withMeChange(next, event.version, me => meAfterSettings(me, snapshot.settings, event.settings));
        }
        case "participant_upserted": {
            const others = snapshot.participants.filter(
                participant => participant.participant_id !== event.participant.participant_id,
            );

            return { ...snapshot, participants: [...others, event.participant] };
        }
        case "column_upserted":
            return { ...snapshot, columns: upsertById(snapshot.columns, event.column) };
        case "column_moved": {
            const moved = snapshot.columns.map(column =>
                column.id === event.column_id ? { ...column, position: event.position } : column,
            );

            return { ...snapshot, columns: withPositions(moved, event.rewritten) };
        }
        case "column_deleted": {
            const removed = cardIdsInColumn(snapshot.cards, event.column_id);
            const cleared = withoutCards(snapshot, removed, event.version);

            return {
                ...cleared,
                columns: cleared.columns.filter(column => column.id !== event.column_id),
                hidden_counts: withoutCount(cleared.hidden_counts, event.column_id),
                spotlight: spotlightWithout(snapshot.spotlight, removed, event.column_id),
            };
        }
        case "column_cleared": {
            const removed = cardIdsInColumn(snapshot.cards, event.column_id);
            const cleared = withoutCards(snapshot, removed, event.version);

            return {
                ...cleared,
                hidden_counts: withoutCount(cleared.hidden_counts, event.column_id),
                spotlight: spotlightWithout(snapshot.spotlight, removed, null),
            };
        }
        case "card_upserted":
            return { ...snapshot, cards: upsertCard(snapshot.cards, event.card) };
        case "card_moved": {
            const moved = snapshot.cards.map(card => {
                if (card.id === event.card_id) {
                    return placedCard(card, event.column_id, event.position, null);
                }

                if (card.parent_card_id === event.card_id) {
                    return { ...card, column_id: event.column_id };
                }

                return card;
            });

            return { ...snapshot, cards: withPositions(moved, event.rewritten) };
        }
        case "card_deleted": {
            const removed = new Set([event.card_id]);

            return {
                ...withoutCards(snapshot, removed, event.version),
                spotlight: spotlightWithout(snapshot.spotlight, removed, null),
            };
        }
        case "cards_merged": {
            const restacked = new Set(event.restacked);
            const merged = snapshot.cards.map(card => {
                if (card.id === event.card_id) {
                    return placedCard(card, event.column_id, event.position, event.parent_card_id);
                }

                if (restacked.has(card.id)) {
                    return { ...card, column_id: event.column_id, parent_card_id: event.parent_card_id };
                }

                return card;
            });

            return { ...snapshot, cards: merged };
        }
        case "card_unmerged": {
            const unmerged = snapshot.cards.map(card =>
                card.id === event.card_id ? placedCard(card, event.column_id, event.position, null) : card,
            );

            return { ...snapshot, cards: withPositions(unmerged, event.rewritten) };
        }
        case "hidden_counts":
            return { ...snapshot, hidden_counts: { ...event.hidden_counts } };
        case "votes_changed":
            return {
                ...snapshot,
                cards: snapshot.cards.map(card =>
                    card.id === event.card_id ? { ...card, votes: { total: event.total } } : card,
                ),
            };
        case "votes_reset": {
            const cards = snapshot.cards.map(card => ({
                ...card,
                votes: { total: card.votes.total === null ? null : 0 },
            }));

            return withMeChange({ ...snapshot, cards }, event.version, me => meAfterReset(me, snapshot.settings));
        }
        case "comment_upserted":
            return { ...snapshot, comments: upsertOwned(snapshot.comments, event.comment) };
        case "comment_deleted":
            return { ...snapshot, comments: snapshot.comments.filter(comment => comment.id !== event.comment_id) };
        case "participant_removed":
            return {
                ...snapshot,
                participants: snapshot.participants.filter(
                    participant => participant.participant_id !== event.participant_id,
                ),
            };
        case "cards_revealed":
            return withRevealedCards(snapshot, event);
        case "cards_hidden":
            return {
                ...withRehiddenCards(snapshot, event.hidden_counts, event.version),
                spotlight: { ...snapshot.spotlight, presenting_card_id: null, highlighted_card_id: null },
            };
        case "stage_changed":
            return { ...snapshot, board: { ...snapshot.board, stage: event.stage } };
        case "timer_changed":
            return {
                ...snapshot,
                timer: event.timer === null ? null : { ...event.timer },
                server_now: event.server_now,
            };
        case "action_item_upserted":
            return { ...snapshot, action_items: upsertById(snapshot.action_items, event.action_item) };
        case "action_item_removed":
            return {
                ...snapshot,
                action_items: snapshot.action_items.filter(item => item.id !== event.action_item_id),
            };
        case "previous_action_upserted":
            return { ...snapshot, previous_actions: upsertById(snapshot.previous_actions, event.previous_action) };
        case "column_revealed":
            return withRevealedColumn(snapshot, event);
        case "column_hidden":
            return withHiddenColumn(snapshot, event.column_id, event.version);
        case "column_votes_reset":
            return withColumnVotesReset(snapshot, event.column_id, event.version);
        case "spotlight_changed":
            return { ...snapshot, spotlight: { ...event.spotlight } };
        case "card_reactions_changed":
            return mapCard(snapshot, event.card_id, card => withReactions(card, event.reactions, event.my_reactions));
        case "card_pinned":
            return mapCard(snapshot, event.card_id, card => withPin(card, event.pinned));
        case "card_image_changed":
            return mapCard(snapshot, event.card_id, card => withImage(card, event.image_path));
        case "board_background_changed":
            return withBackground(snapshot, event.background_path);
        case "survey_upserted":
            return { ...snapshot, surveys: upsertById(snapshot.surveys, event.survey) };
        case "survey_removed":
            return { ...snapshot, surveys: snapshot.surveys.filter(survey => survey.id !== event.survey_id) };
    }
}

export function applyBoardEvents(
    snapshot: BoardSnapshot,
    events: readonly BoardEvent[],
    version: number,
): BoardSnapshot {
    if (events.length === 0 && version === snapshot.version) {
        return snapshot;
    }

    let next = stampMe(snapshot);

    for (const event of events) {
        next = applyBoardEvent(next, event);
    }

    return { ...next, version };
}
