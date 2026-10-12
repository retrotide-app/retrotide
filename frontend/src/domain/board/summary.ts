import type { ActionItemView, BoardSnapshot, CardView, ColumnView, CommentView } from "../../types/api";
import { canExportBoard } from "./capabilities";
import { hiddenCount } from "./selectors";
import { compareByPosition } from "./sort";
import { type CardStack, cardStacks, stackCards } from "./stacks";
import { STAGE_INFO } from "./stages";
import { stackVotes } from "./votes";

export interface SummaryCard {
    card: CardView;
    votes: number | null;
    comments: CommentView[];
    actionItems: ActionItemView[];
}

export interface SummaryStack {
    top: SummaryCard;
    merged: SummaryCard[];
    votes: number | null;
}

export interface SummaryColumn {
    column: ColumnView;
    stacks: SummaryStack[];
    hidden: number;
}

export interface BoardSummary {
    boardId: string;
    title: string;
    stage: string;
    ended: boolean;
    createdAt: string | null;
    participants: number;
    columns: SummaryColumn[];
    otherActions: ActionItemView[];
    exportable: boolean;
}

const ENDED_LABEL = "Ended";

function counted(count: number, one: string, many: string): string {
    return count === 1 ? `1 ${one}` : `${count} ${many}`;
}

export function votesLabel(count: number): string {
    return counted(count, "vote", "votes");
}

export function participantsLabel(count: number): string {
    return counted(count, "participant", "participants");
}

export function hiddenCardsLabel(count: number): string {
    return `${counted(count, "card", "cards")} from others, hidden for now`;
}

function groupedBy<T>(items: readonly T[], keyOf: (item: T) => string | null): Map<string, T[]> {
    const groups = new Map<string, T[]>();

    for (const item of items) {
        const key = keyOf(item);

        if (key === null) {
            continue;
        }

        const group = groups.get(key) ?? [];
        group.push(item);
        groups.set(key, group);
    }

    return groups;
}

function compareByVotes(left: SummaryStack, right: SummaryStack): number {
    const difference = (right.votes ?? -1) - (left.votes ?? -1);

    if (difference !== 0) {
        return difference;
    }

    return compareByPosition(left.top.card, right.top.card);
}

export function boardSummary(snapshot: BoardSnapshot): BoardSummary {
    const { board } = snapshot;
    const commentsByCard = groupedBy(snapshot.comments, comment => comment.card_id);
    const actionsByCard = groupedBy(snapshot.action_items, item => item.source_card_id);

    function summaryCard(card: CardView): SummaryCard {
        return {
            card,
            votes: card.votes.total,
            comments: commentsByCard.get(card.id) ?? [],
            actionItems: actionsByCard.get(card.id) ?? [],
        };
    }

    function summaryStack(stack: CardStack): SummaryStack {
        return {
            top: summaryCard(stack.card),
            merged: stack.merged.map(summaryCard),
            votes: stackVotes(stackCards(stack), snapshot.me).total,
        };
    }

    const columns = snapshot.columns.toSorted(compareByPosition).map(column => {
        const stacks = cardStacks(snapshot.cards.filter(card => card.column_id === column.id)).map(summaryStack);

        return {
            column,
            stacks: stacks.toSorted(compareByVotes),
            hidden: hiddenCount(snapshot.hidden_counts, column.id),
        };
    });

    const placed = new Set(
        columns.flatMap(({ stacks }) =>
            stacks.flatMap(stack => [stack.top.card.id, ...stack.merged.map(merged => merged.card.id)]),
        ),
    );
    const otherActions = snapshot.action_items.filter(
        item => item.source_card_id === null || !placed.has(item.source_card_id),
    );

    return {
        boardId: snapshot.board_id,
        title: board.title,
        stage: board.closed ? ENDED_LABEL : STAGE_INFO[board.stage].label,
        ended: board.closed,
        createdAt: board.created_at ?? null,
        participants: snapshot.participants.length,
        columns,
        otherActions,
        exportable: canExportBoard(snapshot),
    };
}
