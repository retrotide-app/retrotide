import { type QueryClient, useMutation, useQueryClient } from "@tanstack/react-query";
import { joinBoardByLink, rotateShareLink, setBoardPassword, setLinkAccess } from "../../api/endpoints/shareLinks";
import { queryKeys } from "../../api/queryKeys";
import type { JoinBoardRequest, LinkAccess, SharingCommandResponse } from "../../types/api";
import { newId } from "./ids";
import { refreshWhenChanged } from "./refresh";

type SharingSender<TVariables> = (variables: TVariables, opId: string) => Promise<SharingCommandResponse>;

const FORBIDDEN_STATUS = 403;

const GONE_STATUS = 404;

const CONFLICT_STATUS = 409;

const SHARING_OUTDATED_ON = [FORBIDDEN_STATUS, GONE_STATUS, CONFLICT_STATUS];

async function settleSharing(qc: QueryClient, boardId: string, answer: SharingCommandResponse): Promise<void> {
    const key = queryKeys.boardSharing(boardId);

    if (answer.sharing === undefined) {
        await qc.invalidateQueries({ queryKey: key });
        return;
    }

    qc.setQueryData(key, answer.sharing);
}

function useSharingCommand<TVariables>(boardId: string, send: SharingSender<TVariables>) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: (variables: TVariables) => send(variables, newId()),
        onSuccess: answer => settleSharing(qc, boardId, answer),
        onError: error => refreshWhenChanged(qc, error, [queryKeys.boardSharing(boardId)], SHARING_OUTDATED_ON),
    });
}

export function useJoinBoardByLink(slug: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: (request: JoinBoardRequest) => joinBoardByLink(slug, request),
        onSuccess: async joined => {
            await Promise.all([
                qc.invalidateQueries({ queryKey: queryKeys.auth.session() }),
                qc.invalidateQueries({ queryKey: queryKeys.boardLive.state(joined.board_id) }),
            ]);
        },
        onError: error => refreshWhenChanged(qc, error, [queryKeys.shareLinks.preview(slug)], [FORBIDDEN_STATUS]),
    });
}

export function useSetLinkAccess(boardId: string) {
    return useSharingCommand(boardId, (linkAccess: LinkAccess, opId) =>
        setLinkAccess(boardId, { op_id: opId, link_access: linkAccess }),
    );
}

export function useSetBoardPassword(boardId: string) {
    return useSharingCommand(boardId, (password: string | null, opId) =>
        setBoardPassword(boardId, { op_id: opId, password }),
    );
}

export function useRotateShareLink(boardId: string) {
    return useSharingCommand(boardId, (_variables: undefined, opId) => rotateShareLink(boardId, { op_id: opId }));
}
