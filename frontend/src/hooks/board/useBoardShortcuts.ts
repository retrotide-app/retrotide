import { useEffect } from "react";
import { BOARD_LAYOUT_ID, BOARD_SEARCH_ID, type ShortcutAction, shortcutFor } from "../../domain/board/shortcuts";

export interface BoardShortcutOptions {
    active: boolean;
    onHelp: () => void;
}

const DIALOGS = '[role="dialog"], dialog';

const EDITABLE = '[contenteditable=""], [contenteditable="true"]';

const NEW_CARD = "[data-new-card]";

function closestTo(target: EventTarget | null, selector: string): boolean {
    return target instanceof Element && target.closest(selector) !== null;
}

function focusNewCard(): boolean {
    const target = document.querySelector<HTMLElement>(NEW_CARD);

    if (target === null) {
        return false;
    }

    if (target instanceof HTMLFormElement) {
        target.querySelector<HTMLElement>("textarea")?.focus();
        return true;
    }

    target.click();

    return true;
}

function focusSearch(): boolean {
    const search = document.getElementById(BOARD_SEARCH_ID);

    if (search === null) {
        return false;
    }

    search.focus();

    return true;
}

function switchLayout(): boolean {
    const other = document.getElementById(BOARD_LAYOUT_ID)?.querySelector<HTMLElement>('[aria-pressed="false"]');

    if (other === null || other === undefined) {
        return false;
    }

    other.click();
    other.focus();

    return true;
}

function perform(action: ShortcutAction, onHelp: () => void): boolean {
    switch (action) {
        case "help":
            onHelp();
            return true;
        case "new-card":
            return focusNewCard();
        case "search":
            return focusSearch();
        case "layout":
            return switchLayout();
    }
}

export function useBoardShortcuts({ active, onHelp }: BoardShortcutOptions) {
    useEffect(() => {
        if (!active) {
            return;
        }

        function handleKeyDown(event: KeyboardEvent) {
            const { target } = event;

            if (event.defaultPrevented || closestTo(target, DIALOGS)) {
                return;
            }

            const action = shortcutFor({
                key: event.key,
                ctrlKey: event.ctrlKey,
                metaKey: event.metaKey,
                altKey: event.altKey,
                targetTag: target instanceof Element ? target.tagName : "",
                editable: closestTo(target, EDITABLE),
            });

            if (action === null) {
                return;
            }

            if (perform(action, onHelp)) {
                event.preventDefault();
            }
        }

        window.addEventListener("keydown", handleKeyDown);

        return () => window.removeEventListener("keydown", handleKeyDown);
    }, [active, onHelp]);
}
