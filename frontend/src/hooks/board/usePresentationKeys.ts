import { useEffect } from "react";
import type { CardSpotlightChange } from "../mutations/facilitation";

export interface PresentationKeyHandlers {
    presenting: boolean;
    steers: boolean;
    next: string | null;
    previous: string | null;
    onPresent: (change: CardSpotlightChange) => void;
    onLeave: () => void;
}

const TYPING_TARGETS =
    'input, textarea, select, [contenteditable=""], [contenteditable="true"], [role="dialog"], dialog';

function typingIn(target: EventTarget | null): boolean {
    return target instanceof Element && target.closest(TYPING_TARGETS) !== null;
}

function stepFor(key: string, next: string | null, previous: string | null): string | null {
    if (key === "ArrowRight") {
        return next;
    }

    if (key === "ArrowLeft") {
        return previous;
    }

    return null;
}

export function usePresentationKeys({
    presenting,
    steers,
    next,
    previous,
    onPresent,
    onLeave,
}: PresentationKeyHandlers) {
    useEffect(() => {
        if (!presenting) {
            return;
        }

        function handleKeyDown(event: KeyboardEvent) {
            if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || typingIn(event.target)) {
                return;
            }

            if (event.key === "Escape") {
                event.preventDefault();

                if (steers) {
                    onPresent({ cardId: null });
                    return;
                }

                onLeave();
                return;
            }

            if (!steers) {
                return;
            }

            const step = stepFor(event.key, next, previous);

            if (step === null) {
                return;
            }

            event.preventDefault();
            onPresent({ cardId: step });
        }

        window.addEventListener("keydown", handleKeyDown);

        return () => window.removeEventListener("keydown", handleKeyDown);
    }, [presenting, steers, next, previous, onPresent, onLeave]);
}
