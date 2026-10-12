export type ShortcutAction = "help" | "new-card" | "search" | "layout";

export interface ShortcutKey {
    key: string;
    ctrlKey: boolean;
    metaKey: boolean;
    altKey: boolean;
    targetTag: string;
    editable: boolean;
}

export interface ShortcutEntry {
    keys: string;
    description: string;
}

export const BOARD_SEARCH_ID = "board-card-search";

export const BOARD_LAYOUT_ID = "board-layout-toggle";

export const SHORTCUTS: readonly ShortcutEntry[] = [
    { keys: "?", description: "Show these keyboard shortcuts" },
    { keys: "N", description: "Write a new card in the first column you can add to" },
    { keys: "/", description: "Search the cards" },
    { keys: "L", description: "Switch between the columns and list layouts" },
    { keys: "Esc", description: "Close the open dialog" },
];

const TYPING_TAGS: ReadonlySet<string> = new Set(["INPUT", "TEXTAREA", "SELECT"]);

const ACTIONS: ReadonlyMap<string, ShortcutAction> = new Map<string, ShortcutAction>([
    ["?", "help"],
    ["n", "new-card"],
    ["/", "search"],
    ["l", "layout"],
]);

export function shortcutFor(event: ShortcutKey): ShortcutAction | null {
    if (event.ctrlKey || event.metaKey || event.altKey) {
        return null;
    }

    if (event.editable || TYPING_TAGS.has(event.targetTag.toUpperCase())) {
        return null;
    }

    return ACTIONS.get(event.key) ?? null;
}
