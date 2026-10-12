import { createContext, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useDeleteCard } from "../mutations/boards";

export interface CardDeleteUndo {
    hidden: ReadonlySet<string>;
    waiting: readonly string[];
    schedule: (cardId: string) => void;
    undo: (cardId: string) => void;
    error: Error | null;
    dismissError: () => void;
}

export const CARD_DELETE_UNDO_MS = 6000;

export const CardDeleteUndoContext = createContext<CardDeleteUndo | null>(null);

function without(ids: readonly string[], cardId: string): string[] {
    return ids.filter(id => id !== cardId);
}

function asError(failure: unknown): Error {
    return failure instanceof Error ? failure : new Error(String(failure), { cause: failure });
}

export function useDeferredCardDelete(boardId: string): CardDeleteUndo {
    const { mutate, mutateAsync } = useDeleteCard(boardId);
    const [waiting, setWaiting] = useState<readonly string[]>([]);
    const [sent, setSent] = useState<readonly string[]>([]);
    const [error, setError] = useState<Error | null>(null);
    const timers = useRef(new Map<string, number>());
    const senders = useRef({ mutate, mutateAsync });

    useEffect(() => {
        senders.current = { mutate, mutateAsync };
    }, [mutate, mutateAsync]);

    const commit = useCallback((cardId: string) => {
        timers.current.delete(cardId);
        setWaiting(held => without(held, cardId));
        setSent(held => [...held, cardId]);

        senders.current.mutateAsync({ cardId }).catch((failure: unknown) => {
            setSent(held => without(held, cardId));
            setError(asError(failure));
        });
    }, []);

    const schedule = useCallback(
        (cardId: string) => {
            if (timers.current.has(cardId)) {
                return;
            }

            setError(null);
            timers.current.set(
                cardId,
                window.setTimeout(() => commit(cardId), CARD_DELETE_UNDO_MS),
            );
            setWaiting(held => [...held, cardId]);
        },
        [commit],
    );

    const undo = useCallback((cardId: string) => {
        const timer = timers.current.get(cardId);

        if (timer === undefined) {
            return;
        }

        window.clearTimeout(timer);
        timers.current.delete(cardId);
        setWaiting(held => without(held, cardId));
    }, []);

    const dismissError = useCallback(() => setError(null), []);

    useEffect(() => {
        const held = timers.current;

        return () => {
            for (const [cardId, timer] of held) {
                window.clearTimeout(timer);
                senders.current.mutate({ cardId });
            }

            held.clear();
        };
    }, []);

    const hidden = useMemo(() => new Set([...waiting, ...sent]), [waiting, sent]);

    return useMemo(
        () => ({ hidden, waiting, schedule, undo, error, dismissError }),
        [hidden, waiting, schedule, undo, error, dismissError],
    );
}
