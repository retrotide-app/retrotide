import type { PropsWithChildren } from "react";
import { CardDeleteUndoContext, useDeferredCardDelete } from "../hooks/board/useDeferredCardDelete";

interface CardDeleteUndoProviderProps {
    boardId: string;
}

export function CardDeleteUndoProvider({ boardId, children }: PropsWithChildren<CardDeleteUndoProviderProps>) {
    const deferred = useDeferredCardDelete(boardId);

    return <CardDeleteUndoContext.Provider value={deferred}>{children}</CardDeleteUndoContext.Provider>;
}
