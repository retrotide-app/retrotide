import { type Mutation, type QueryClient, useMutation, useMutationState, useQueryClient } from "@tanstack/react-query";
import {
    castVote,
    changeStage,
    clearColumn,
    closeBoard,
    createBoard,
    createCard,
    createColumn,
    createComment,
    deleteCard,
    deleteColumn,
    deleteComment,
    describeBoard,
    editCard,
    editComment,
    hideCards,
    joinBoard,
    mergeCard,
    moveCard,
    moveColumn,
    recolourCard,
    removeParticipant,
    removeVote,
    resetVotes,
    revealCards,
    runTimer,
    setParticipantRole,
    unmergeCard,
    updateBoardSettings,
    updateColumn,
} from "../../api/endpoints/boards";
import { queryKeys } from "../../api/queryKeys";
import type { PendingChange, PendingCommand } from "../../domain/board/optimistic";
import { withMe } from "../../domain/board/votes";
import type {
    BoardCommandResponse,
    BoardSettingsRequest,
    BoardSnapshot,
    ColumnFieldsRequest,
    CreateBoardRequest,
    CreateCardRequest,
    CreateColumnRequest,
    CreateCommentRequest,
    DescribeBoardRequest,
    EditCardRequest,
    EditCommentRequest,
    MoveCardRequest,
    MoveColumnRequest,
    ParticipantRoleRequest,
    RecolourCardRequest,
    StageRequest,
    TimerRequest,
} from "../../types/api";
import {
    BOARD_OUTDATED_ON,
    type CommandOptions,
    CONFLICT_STATUS,
    GONE_STATUS,
    OPTIMISTIC_META_KEY,
    useBoardCommand,
} from "./boardCommand";
import { newId } from "./ids";
import { refreshWhenChanged } from "./refresh";

export interface BoardOrganisationTarget {
    orgId: string;
}

export type BoardCreation = BoardOrganisationTarget & CreateBoardRequest;

export type BoardDescription = Omit<DescribeBoardRequest, "op_id">;

export type ColumnDraft = Omit<CreateColumnRequest, "op_id" | "id">;

export interface ColumnTarget {
    columnId: string;
}

export type ColumnUpdate = ColumnTarget & Omit<ColumnFieldsRequest, "op_id">;

export type ColumnMove = ColumnTarget & Omit<MoveColumnRequest, "op_id">;

export type CardDraft = Omit<CreateCardRequest, "op_id" | "id">;

export interface CardTarget {
    cardId: string;
}

export type CardEdit = CardTarget & Omit<EditCardRequest, "op_id">;

export type CardRecolour = CardTarget & Omit<RecolourCardRequest, "op_id">;

export type CardMove = CardTarget & Omit<MoveCardRequest, "op_id">;

export interface CardMerge extends CardTarget {
    targetCardId: string;
}

export type CommentDraft = CardTarget & Omit<CreateCommentRequest, "op_id" | "id">;

export interface CommentTarget {
    commentId: string;
}

export type CommentEdit = CommentTarget & Omit<EditCommentRequest, "op_id">;

export type BoardSettingsChange = Omit<BoardSettingsRequest, "op_id">;

export type TimerChange = Omit<TimerRequest, "op_id">;

export type StageChange = Omit<StageRequest, "op_id">;

export interface ParticipantTarget {
    participantId: string;
}

export type ParticipantRoleChange = ParticipantTarget & Omit<ParticipantRoleRequest, "op_id">;

function keepMyVotes(qc: QueryClient, boardId: string, response: BoardCommandResponse): void {
    const { me, version } = response;

    if (me === undefined) {
        return;
    }

    qc.setQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId), held =>
        held === undefined ? held : withMe(held, me, version),
    );
}

const VOTE_OPTIONS_CAST: CommandOptions = { optimistic: "vote-cast", answered: keepMyVotes };

const VOTE_OPTIONS_REMOVE: CommandOptions = { optimistic: "vote-remove", answered: keepMyVotes };

const MENTIONED_OPTIONS: CommandOptions = {
    answered: qc => qc.invalidateQueries({ queryKey: queryKeys.auth.mentions() }),
};

function pendingChange(kind: unknown, variables: unknown): PendingChange | null {
    if (kind === "card-move") {
        const { cardId, column_id: columnId, after_card_id: afterCardId } = variables as CardMove;

        return { kind, cardId, drop: { column_id: columnId, after_card_id: afterCardId } };
    }

    if (kind === "column-move") {
        const { columnId, after_column_id: afterColumnId } = variables as ColumnMove;

        return { kind, columnId, drop: { after_column_id: afterColumnId } };
    }

    if (kind === "vote-cast" || kind === "vote-remove") {
        const { cardId } = variables as CardTarget;

        return { kind: "vote", cardId, delta: kind === "vote-cast" ? 1 : -1 };
    }

    return null;
}

function pendingCommand(mutation: Mutation): PendingCommand | null {
    const { status, variables, data } = mutation.state;

    if (status !== "pending" && status !== "success") {
        return null;
    }

    const change = pendingChange(mutation.options.meta?.[OPTIMISTIC_META_KEY], variables);

    if (change === null) {
        return null;
    }

    const answeredAt = status === "success" ? (data as BoardCommandResponse).version : null;

    return { change, answeredAt };
}

export function usePendingBoardCommands(boardId: string): (PendingCommand | null)[] {
    return useMutationState({
        filters: { mutationKey: queryKeys.boardLive.mutations(boardId) },
        select: pendingCommand,
    });
}

export function useCreateBoard() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ orgId, ...body }: BoardCreation) => createBoard(orgId, body),
        onSuccess: snapshot => {
            qc.setQueryData(queryKeys.boardLive.state(snapshot.board_id), snapshot);
        },
        onSettled: (_data, error, { team_id: teamId }) =>
            refreshWhenChanged(
                qc,
                error,
                teamId === undefined
                    ? [queryKeys.boards.organisations]
                    : [queryKeys.boards.team(teamId), queryKeys.boards.organisations],
                [GONE_STATUS, CONFLICT_STATUS],
            ),
    });
}

export function useJoinBoard(boardId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: () => joinBoard(boardId),
        onSettled: (_data, error) =>
            refreshWhenChanged(qc, error, [queryKeys.boardLive.state(boardId)], BOARD_OUTDATED_ON),
    });
}

export function useDescribeBoard(boardId: string) {
    return useBoardCommand(boardId, (body: BoardDescription, opId) => describeBoard(boardId, { ...body, op_id: opId }));
}

export function useCreateColumn(boardId: string) {
    return useBoardCommand(boardId, (draft: ColumnDraft, opId) =>
        createColumn(boardId, { ...draft, op_id: opId, id: newId() }),
    );
}

export function useUpdateColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId, ...fields }: ColumnUpdate, opId) =>
        updateColumn(boardId, columnId, { ...fields, op_id: opId }),
    );
}

export function useMoveColumn(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ columnId, ...drop }: ColumnMove, opId) => moveColumn(boardId, columnId, { ...drop, op_id: opId }),
        { optimistic: "column-move" },
    );
}

export function useClearColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnTarget, opId) =>
        clearColumn(boardId, columnId, { op_id: opId }),
    );
}

export function useDeleteColumn(boardId: string) {
    return useBoardCommand(boardId, ({ columnId }: ColumnTarget, opId) =>
        deleteColumn(boardId, columnId, { op_id: opId }),
    );
}

export function useCreateCard(boardId: string) {
    return useBoardCommand(
        boardId,
        (draft: CardDraft, opId) => createCard(boardId, { ...draft, op_id: opId, id: newId() }),
        MENTIONED_OPTIONS,
    );
}

export function useEditCard(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ cardId, ...edit }: CardEdit, opId) => editCard(boardId, cardId, { ...edit, op_id: opId }),
        MENTIONED_OPTIONS,
    );
}

export function useRecolourCard(boardId: string) {
    return useBoardCommand(boardId, ({ cardId, ...recolour }: CardRecolour, opId) =>
        recolourCard(boardId, cardId, { ...recolour, op_id: opId }),
    );
}

export function useMoveCard(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ cardId, ...drop }: CardMove, opId) => moveCard(boardId, cardId, { ...drop, op_id: opId }),
        { optimistic: "card-move" },
    );
}

export function useDeleteCard(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ cardId }: CardTarget, opId) => deleteCard(boardId, cardId, { op_id: opId }),
        MENTIONED_OPTIONS,
    );
}

export function useCastVote(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ cardId }: CardTarget, opId) => castVote(boardId, cardId, { op_id: opId }),
        VOTE_OPTIONS_CAST,
    );
}

export function useRemoveVote(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ cardId }: CardTarget, opId) => removeVote(boardId, cardId, { op_id: opId }),
        VOTE_OPTIONS_REMOVE,
    );
}

export function useResetVotes(boardId: string) {
    return useBoardCommand(boardId, (_variables: void, opId) => resetVotes(boardId, { op_id: opId }));
}

export function useUpdateBoardSettings(boardId: string) {
    return useBoardCommand(boardId, (change: BoardSettingsChange, opId) =>
        updateBoardSettings(boardId, { ...change, op_id: opId }),
    );
}

export function useMergeCard(boardId: string) {
    return useBoardCommand(boardId, ({ cardId, targetCardId }: CardMerge, opId) =>
        mergeCard(boardId, cardId, { op_id: opId, target_card_id: targetCardId }),
    );
}

export function useUnmergeCard(boardId: string) {
    return useBoardCommand(boardId, ({ cardId }: CardTarget, opId) => unmergeCard(boardId, cardId, { op_id: opId }));
}

export function useCreateComment(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ cardId, ...draft }: CommentDraft, opId) =>
            createComment(boardId, cardId, { ...draft, op_id: opId, id: newId() }),
        MENTIONED_OPTIONS,
    );
}

export function useEditComment(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ commentId, ...edit }: CommentEdit, opId) => editComment(boardId, commentId, { ...edit, op_id: opId }),
        MENTIONED_OPTIONS,
    );
}

export function useRevealCards(boardId: string) {
    return useBoardCommand(boardId, (_variables: void, opId) => revealCards(boardId, { op_id: opId }));
}

export function useHideCards(boardId: string) {
    return useBoardCommand(boardId, (_variables: void, opId) => hideCards(boardId, { op_id: opId }));
}

export function useChangeStage(boardId: string) {
    return useBoardCommand(boardId, ({ stage }: StageChange, opId) => changeStage(boardId, { op_id: opId, stage }));
}

export function useBoardTimerCommand(boardId: string) {
    return useBoardCommand(boardId, (change: TimerChange, opId) => runTimer(boardId, { ...change, op_id: opId }));
}

export function useCloseBoard(boardId: string) {
    return useBoardCommand(boardId, (_variables: void, opId) => closeBoard(boardId, { op_id: opId }));
}

export function useSetParticipantRole(boardId: string) {
    return useBoardCommand(boardId, ({ participantId, role }: ParticipantRoleChange, opId) =>
        setParticipantRole(boardId, participantId, { op_id: opId, role }),
    );
}

export function useRemoveParticipant(boardId: string) {
    return useBoardCommand(boardId, ({ participantId }: ParticipantTarget, opId) =>
        removeParticipant(boardId, participantId, { op_id: opId }),
    );
}

export function useDeleteComment(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ commentId }: CommentTarget, opId) => deleteComment(boardId, commentId, { op_id: opId }),
        MENTIONED_OPTIONS,
    );
}
