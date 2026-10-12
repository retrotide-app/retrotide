import {
    focusColumn,
    hideColumn,
    highlightCard,
    presentCard,
    resetColumnVotes,
    revealColumn,
    setColumnVoteLimit,
    showColumn,
} from "../../api/endpoints/boards";
import { useBoardCommand } from "./boardCommand";
import type { ColumnTarget } from "./boards";

export type ColumnVoteLimitChange = ColumnTarget & { maxVotes: number | null };

export interface CardSpotlightChange {
    cardId: string | null;
}

export interface ColumnFocusChange {
    columnId: string | null;
}

export function useRevealColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnTarget, opId) =>
        revealColumn(boardId, columnId, { op_id: opId }),
    );
}

export function useHideColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnTarget, opId) =>
        hideColumn(boardId, columnId, { op_id: opId }),
    );
}

export function useShowColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnTarget, opId) =>
        showColumn(boardId, columnId, { op_id: opId }),
    );
}

export function useResetColumnVotes(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnTarget, opId) =>
        resetColumnVotes(boardId, columnId, { op_id: opId }),
    );
}

export function useSetColumnVoteLimit(boardId: string) {
    return useBoardCommand(boardId, ({ columnId, maxVotes }: ColumnVoteLimitChange, opId) =>
        setColumnVoteLimit(boardId, columnId, { op_id: opId, max_votes: maxVotes }),
    );
}

export function usePresentCard(boardId: string) {
    return useBoardCommand(boardId, ({ cardId }: CardSpotlightChange, opId) =>
        presentCard(boardId, { op_id: opId, card_id: cardId }),
    );
}

export function useHighlightCard(boardId: string) {
    return useBoardCommand(boardId, ({ cardId }: CardSpotlightChange, opId) =>
        highlightCard(boardId, { op_id: opId, card_id: cardId }),
    );
}

export function useFocusColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnFocusChange, opId) =>
        focusColumn(boardId, { op_id: opId, column_id: columnId }),
    );
}
