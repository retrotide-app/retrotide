import { useQuery } from "@tanstack/react-query";
import { getBoardSharing, getJoinPreview } from "../../api/endpoints/shareLinks";
import { queryKeys } from "../../api/queryKeys";
import type { BoardSharingResponse, JoinPreviewResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useJoinPreview(slug: string): QueryState<JoinPreviewResponse> {
    const preview = useQuery({
        queryKey: queryKeys.shareLinks.preview(slug),
        queryFn: () => getJoinPreview(slug),
        enabled: slug !== "",
        staleTime: 0,
    });

    return queryState(preview);
}

export function useBoardSharing(boardId: string): QueryState<BoardSharingResponse> {
    const sharing = useQuery({
        queryKey: queryKeys.boardSharing(boardId),
        queryFn: () => getBoardSharing(boardId),
        enabled: boardId !== "",
    });

    return queryState(sharing);
}
