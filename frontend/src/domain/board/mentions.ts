import type { MentionKind, MentionView, PersonView } from "../../types/api";

export interface MentionQuery {
    start: number;
    end: number;
    text: string;
}

export interface TitleEdit {
    title: string;
    caret: number;
    mentions: PersonView[];
}

export interface TitlePart {
    text: string;
    mention: PersonView | null;
}

export const MAX_MENTIONS = 20;

export const MAX_SUGGESTIONS = 6;

export const NO_MENTIONS: readonly PersonView[] = [];

export const MENTION_KIND_LABELS: Readonly<Record<MentionKind, string>> = {
    action_item: "Action item",
    card: "Card",
    comment: "Comment",
};

const MENTION_SIGN = "@";

const MENTION_HINT = "Type @ to mention someone";

const ENDS_SENTENCE = /[.?!]$/;

const MAX_QUERY_LENGTH = 40;

const BREAKS = /[\n@]/;

function startsWord(text: string, index: number): boolean {
    return index === 0 || /\s/.test(text.charAt(index - 1));
}

export function mentionQuery(text: string, caret: number): MentionQuery | null {
    const before = text.slice(0, caret);
    const start = before.lastIndexOf(MENTION_SIGN);

    if (start < 0 || !startsWord(text, start)) {
        return null;
    }

    const query = before.slice(start + 1);

    if (query.length > MAX_QUERY_LENGTH || BREAKS.test(query) || query.startsWith(" ")) {
        return null;
    }

    return { start, end: caret, text: query };
}

function matches(person: PersonView, query: string): boolean {
    const name = person.display_name.toLocaleLowerCase();

    if (name.startsWith(query)) {
        return true;
    }

    return name.split(/\s+/).some(word => word.startsWith(query));
}

export function suggestPeople(people: readonly PersonView[], query: MentionQuery | null): PersonView[] {
    if (query === null) {
        return [];
    }

    const wanted = query.text.toLocaleLowerCase();

    return people.filter(person => matches(person, wanted)).slice(0, MAX_SUGGESTIONS);
}

function withPerson(mentions: readonly PersonView[], person: PersonView): PersonView[] {
    if (mentions.some(held => held.user_id === person.user_id)) {
        return [...mentions];
    }

    return [...mentions, person];
}

export function insertMention(
    title: string,
    query: MentionQuery,
    person: PersonView,
    mentions: readonly PersonView[],
): TitleEdit {
    const inserted = `${MENTION_SIGN}${person.display_name} `;
    const next = `${title.slice(0, query.start)}${inserted}${title.slice(query.end)}`;

    return { title: next, caret: query.start + inserted.length, mentions: withPerson(mentions, person) };
}

function mentionedIn(title: string, person: PersonView): boolean {
    return title.includes(`${MENTION_SIGN}${person.display_name}`);
}

export function keptMentions(title: string, mentions: readonly PersonView[]): PersonView[] {
    const kept: PersonView[] = [];

    for (const person of mentions) {
        if (mentionedIn(title, person) && !kept.some(held => held.user_id === person.user_id)) {
            kept.push(person);
        }
    }

    return kept;
}

export function mentionsChanged(held: readonly PersonView[], ids: readonly string[]): boolean {
    const wanted = new Set(ids);

    if (wanted.size !== held.length) {
        return true;
    }

    return held.some(person => !wanted.has(person.user_id));
}

export function mentionPlaceholder(placeholder: string | undefined, picking: boolean): string | undefined {
    if (!picking) {
        return placeholder;
    }

    if (placeholder === undefined) {
        return MENTION_HINT;
    }

    const joiner = ENDS_SENTENCE.test(placeholder) ? " " : ". ";

    return `${placeholder}${joiner}${MENTION_HINT}`;
}

export function mentionKey(mention: MentionView): string {
    switch (mention.kind) {
        case "action_item":
            return `action_item:${mention.action_item_id}`;
        case "card":
            return `card:${mention.card_id}`;
        case "comment":
            return `comment:${mention.comment_id}`;
    }
}

export function mentionText(mention: MentionView): string {
    return mention.kind === "action_item" ? mention.title : mention.body;
}

function longestFirst(mentions: readonly PersonView[]): PersonView[] {
    return mentions.toSorted((left, right) => right.display_name.length - left.display_name.length);
}

function nextMention(title: string, from: number, mentions: readonly PersonView[]): [number, PersonView] | null {
    let found: [number, PersonView] | null = null;

    for (const person of mentions) {
        const at = title.indexOf(`${MENTION_SIGN}${person.display_name}`, from);

        if (at >= 0 && (found === null || at < found[0])) {
            found = [at, person];
        }
    }

    return found;
}

export function titleParts(title: string, mentions: readonly PersonView[]): TitlePart[] {
    const named = longestFirst(mentions.filter(person => person.display_name !== ""));
    const parts: TitlePart[] = [];
    let from = 0;

    for (let found = nextMention(title, from, named); found !== null; found = nextMention(title, from, named)) {
        const [at, person] = found;

        if (at > from) {
            parts.push({ text: title.slice(from, at), mention: null });
        }

        const text = `${MENTION_SIGN}${person.display_name}`;
        parts.push({ text, mention: person });
        from = at + text.length;
    }

    if (from < title.length) {
        parts.push({ text: title.slice(from), mention: null });
    }

    return parts;
}
