import { useEffect, useMemo } from "react";
import { rejoinSlugFrom } from "../../domain/board/shareLink";
import { readBoardJoinLink, writeBoardJoinLink } from "../../platform/boardJoinLink";

export function useRememberedJoinLink(boardId: string, routeState: unknown): string | null {
    const fromRoute = rejoinSlugFrom(routeState);

    useEffect(() => {
        if (fromRoute !== null) {
            writeBoardJoinLink(boardId, fromRoute);
        }
    }, [boardId, fromRoute]);

    return useMemo(() => fromRoute ?? readBoardJoinLink(boardId), [boardId, fromRoute]);
}
