import { type QueryClient, useMutation, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "../../api/queryKeys";
import type { BoardCommandResponse } from "../../types/api";
import { newId } from "./ids";
import { refreshWhenChanged } from "./refresh";

export type CommandSender<TVariables> = (variables: TVariables, opId: string) => Promise<BoardCommandResponse>;

export type CommandAnswered = (qc: QueryClient, boardId: string, response: BoardCommandResponse) => void;

export type OptimisticKind = "card-move" | "column-move" | "vote-cast" | "vote-remove";

export interface CommandOptions {
    optimistic?: OptimisticKind;
    answered?: CommandAnswered;
}

export const OPTIMISTIC_META_KEY = "optimistic";

export const FORBIDDEN_STATUS = 403;

export const GONE_STATUS = 404;

export const CONFLICT_STATUS = 409;

export const BOARD_OUTDATED_ON = [FORBIDDEN_STATUS, GONE_STATUS, CONFLICT_STATUS];

export function useBoardCommand<TVariables>(
    boardId: string,
    send: CommandSender<TVariables>,
    options: CommandOptions = {},
) {
    const qc = useQueryClient();
    const { optimistic, answered } = options;

    return useMutation({
        mutationKey: queryKeys.boardLive.mutations(boardId),
        mutationFn: (variables: TVariables) => send(variables, newId()),
        meta: optimistic === undefined ? undefined : { [OPTIMISTIC_META_KEY]: optimistic },
        onSuccess: response => answered?.(qc, boardId, response),
        onError: error => refreshWhenChanged(qc, error, [queryKeys.boardLive.state(boardId)], BOARD_OUTDATED_ON),
    });
}
