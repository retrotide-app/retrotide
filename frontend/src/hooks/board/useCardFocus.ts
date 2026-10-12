import { useState } from "react";

export interface CardFocus {
    request: (cardId: string | null) => void;
    claim: (cardId: string) => boolean;
    reveal: (cardId: string) => boolean;
    watch: (cardId: string, onReveal: () => void) => () => void;
}

export function createCardFocus(): CardFocus {
    let pending: string | null = null;
    const watchers = new Map<string, () => void>();

    return {
        request: cardId => {
            pending = cardId;
        },
        claim: cardId => {
            if (pending !== cardId) {
                return false;
            }

            pending = null;

            return true;
        },
        reveal: cardId => {
            const onReveal = watchers.get(cardId);

            if (onReveal === undefined) {
                return false;
            }

            onReveal();

            return true;
        },
        watch: (cardId, onReveal) => {
            watchers.set(cardId, onReveal);

            return () => {
                if (watchers.get(cardId) === onReveal) {
                    watchers.delete(cardId);
                }
            };
        },
    };
}

export function useCardFocus(): CardFocus {
    const [cardFocus] = useState(createCardFocus);

    return cardFocus;
}
