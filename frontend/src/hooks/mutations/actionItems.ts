import { type QueryClient, useMutation, useQueryClient } from "@tanstack/react-query";
import {
    addActionItem,
    deleteActionItem,
    editActionItem,
    markActionItem,
    reviewPreviousAction,
} from "../../api/endpoints/actionItems";
import { queryKeys } from "../../api/queryKeys";
import type { ActionItemFieldsRequest, ActionItemStatus, ReviewOutcome } from "../../types/api";
import { BOARD_OUTDATED_ON, type CommandOptions, useBoardCommand } from "./boardCommand";
import { newId } from "./ids";
import { refreshWhenChanged } from "./refresh";

export type ActionItemDraft = Omit<ActionItemFieldsRequest, "op_id"> & { source_card_id: string };

export interface ActionItemTarget {
    itemId: string;
}

export type ActionItemEdit = ActionItemTarget & Omit<ActionItemFieldsRequest, "op_id">;

export interface ActionItemMark extends ActionItemTarget {
    status: ActionItemStatus;
}

export interface PreviousActionReview extends ActionItemTarget {
    outcome: ReviewOutcome;
}

export interface ListedActionItemMark extends ActionItemMark {
    boardId: string;
}

async function refreshListings(qc: QueryClient): Promise<void> {
    await Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.actionItems.all }),
        qc.invalidateQueries({ queryKey: queryKeys.auth.mentions() }),
    ]);
}

const LISTED_OPTIONS: CommandOptions = { answered: refreshListings };

export function useAddActionItem(boardId: string) {
    return useBoardCommand(
        boardId,
        (draft: ActionItemDraft, opId) => addActionItem(boardId, { ...draft, op_id: opId, id: newId() }),
        LISTED_OPTIONS,
    );
}

export function useEditActionItem(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ itemId, ...fields }: ActionItemEdit, opId) => editActionItem(boardId, itemId, { ...fields, op_id: opId }),
        LISTED_OPTIONS,
    );
}

export function useMarkActionItem(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ itemId, status }: ActionItemMark, opId) => markActionItem(boardId, itemId, { op_id: opId, status }),
        LISTED_OPTIONS,
    );
}

export function useDeleteActionItem(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ itemId }: ActionItemTarget, opId) => deleteActionItem(boardId, itemId, { op_id: opId }),
        LISTED_OPTIONS,
    );
}

export function useReviewPreviousAction(boardId: string) {
    return useBoardCommand(
        boardId,
        ({ itemId, outcome }: PreviousActionReview, opId) =>
            reviewPreviousAction(boardId, itemId, { op_id: opId, outcome }),
        LISTED_OPTIONS,
    );
}

export function useMarkListedActionItem(orgId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ boardId, itemId, status }: ListedActionItemMark) =>
            markActionItem(boardId, itemId, { op_id: newId(), status }),
        onSettled: (_data, error, { boardId }) =>
            refreshWhenChanged(
                qc,
                error,
                [
                    queryKeys.actionItems.organisation(orgId),
                    queryKeys.auth.mentions(),
                    queryKeys.boardLive.state(boardId),
                ],
                BOARD_OUTDATED_ON,
            ),
    });
}
