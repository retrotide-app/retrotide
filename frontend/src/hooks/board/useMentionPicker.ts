import {
    type ChangeEvent,
    type KeyboardEvent,
    type RefObject,
    type SyntheticEvent,
    useId,
    useLayoutEffect,
    useRef,
    useState,
} from "react";
import { insertMention, mentionQuery, suggestPeople } from "../../domain/board/mentions";
import type { PersonView } from "../../types/api";

export type MentionElement = HTMLInputElement | HTMLTextAreaElement;

export interface MentionPickerInput<T extends MentionElement> {
    text: string;
    mentions: readonly PersonView[];
    people: readonly PersonView[] | null;
    onChange: (text: string, mentions: PersonView[]) => void;
    onKeyDown?: (e: KeyboardEvent<T>) => void;
}

export interface MentionCombobox<T extends MentionElement> {
    ref: RefObject<T | null>;
    role: "combobox";
    "aria-autocomplete": "list";
    "aria-expanded": boolean;
    "aria-controls": string | undefined;
    "aria-activedescendant": string | undefined;
    autoComplete: "off";
    value: string;
    onChange: (e: ChangeEvent<T>) => void;
    onSelect: (e: SyntheticEvent<T>) => void;
    onKeyDown: (e: KeyboardEvent<T>) => void;
    onBlur: () => void;
    onFocus: () => void;
}

export interface MentionPicker<T extends MentionElement> {
    combobox: MentionCombobox<T>;
    listId: string;
    open: boolean;
    suggestions: PersonView[];
    active: number;
    optionIds: string[];
    pick: (person: PersonView) => void;
}

const NO_PEOPLE: readonly PersonView[] = [];

export function useMentionPicker<T extends MentionElement>({
    text,
    mentions,
    people,
    onChange,
    onKeyDown,
}: MentionPickerInput<T>): MentionPicker<T> {
    const listId = useId();
    const ref = useRef<T>(null);
    const pendingCaretRef = useRef<number | null>(null);
    const [caret, setCaret] = useState(text.length);
    const [highlighted, setHighlighted] = useState(0);
    const [dismissedAt, setDismissedAt] = useState<number | null>(null);

    const query = mentionQuery(text, caret);
    const suggestions = suggestPeople(people ?? NO_PEOPLE, query);
    const open = query !== null && suggestions.length > 0 && dismissedAt !== query.start;
    const active = Math.min(highlighted, suggestions.length - 1);
    const optionIds = suggestions.map((_person, index) => `${listId}-${index}`);

    useLayoutEffect(() => {
        const pending = pendingCaretRef.current;

        if (pending === null) {
            return;
        }

        pendingCaretRef.current = null;
        ref.current?.setSelectionRange(pending, pending);
    });

    function pick(person: PersonView) {
        if (query === null) {
            return;
        }

        const edit = insertMention(text, query, person, mentions);
        pendingCaretRef.current = edit.caret;
        setCaret(edit.caret);
        setHighlighted(0);
        setDismissedAt(query.start);
        onChange(edit.title, edit.mentions);
    }

    function handleKeyDown(e: KeyboardEvent<T>) {
        if (!open || query === null) {
            onKeyDown?.(e);
            return;
        }

        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            const step = e.key === "ArrowDown" ? 1 : -1;

            setHighlighted((active + step + suggestions.length) % suggestions.length);
            return;
        }

        if (e.key === "Enter" || e.key === "Tab") {
            e.preventDefault();
            pick(suggestions[active]);
            return;
        }

        if (e.key === "Escape") {
            e.preventDefault();
            e.stopPropagation();
            setDismissedAt(query.start);
            return;
        }

        onKeyDown?.(e);
    }

    return {
        combobox: {
            ref,
            role: "combobox",
            "aria-autocomplete": "list",
            "aria-expanded": open,
            "aria-controls": open ? listId : undefined,
            "aria-activedescendant": open ? optionIds[active] : undefined,
            autoComplete: "off",
            value: text,
            onChange: e => {
                setCaret(e.target.selectionStart ?? e.target.value.length);
                setHighlighted(0);
                setDismissedAt(null);
                onChange(e.target.value, [...mentions]);
            },
            onSelect: e => setCaret(e.currentTarget.selectionStart ?? text.length),
            onKeyDown: handleKeyDown,
            onBlur: () => setDismissedAt(query?.start ?? null),
            onFocus: () => setDismissedAt(null),
        },
        listId,
        open,
        suggestions,
        active,
        optionIds,
        pick,
    };
}
