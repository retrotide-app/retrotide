import type {
    ActionItemCountsView,
    ActionItemListItem,
    ActionItemStatus,
    ActionItemView,
    BoardSnapshot,
    CardView,
    PersonView,
    ReviewOutcome,
    TeamResponse,
} from "../../types/api";
import { clampChars } from "../../utils/text";

export type StatusFilter = "all" | ActionItemStatus;

export type AssigneeFilter = "anyone" | "me" | "none" | string;

export type TeamFilter = "all" | "organisation" | string;

export interface StatusFilterOption {
    value: StatusFilter;
    label: string;
}

export interface FilterChoice {
    value: string;
    label: string;
}

const FIXED_ASSIGNEE_CHOICES: readonly FilterChoice[] = [
    { value: "anyone", label: "Anyone" },
    { value: "me", label: "Assigned to me" },
    { value: "none", label: "Unassigned" },
];

const FIXED_TEAM_CHOICES: readonly FilterChoice[] = [
    { value: "all", label: "All boards" },
    { value: "organisation", label: "Organisation boards" },
];

export const ACTION_ITEM_TITLE_MAX_LENGTH = 500;

export const ACTION_ITEM_STATUS_LABELS: Readonly<Record<ActionItemStatus, string>> = {
    open: "Open",
    done: "Done",
    dropped: "Dropped",
};

export const REVIEW_OUTCOME_LABELS: Readonly<Record<ReviewOutcome, string>> = {
    done: "Done",
    carried: "Carried over",
    dropped: "Dropped",
};

export const REVIEW_OUTCOMES: readonly ReviewOutcome[] = ["done", "carried", "dropped"];

const DUE_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

const DUE_DATE_FORMAT = new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
});

export function formatDueDate(dueOn: string | null): string {
    if (dueOn === null) {
        return "";
    }

    const parts = DUE_DATE.exec(dueOn);

    if (parts === null) {
        return dueOn;
    }

    const [, year, month, day] = parts;

    return DUE_DATE_FORMAT.format(new Date(Date.UTC(Number(year), Number(month) - 1, Number(day))));
}

export function titleFromCard(body: string): string {
    const line = body.replace(/\s+/g, " ").trim();

    return clampChars(line, ACTION_ITEM_TITLE_MAX_LENGTH);
}

export function dueOnFrom(value: string): string | null {
    const trimmed = value.trim();

    return trimmed === "" ? null : trimmed;
}

export function cardActionCount(snapshot: BoardSnapshot, stack: readonly CardView[]): number {
    const ids = new Set(stack.map(card => card.id));

    return snapshot.action_items.filter(item => item.source_card_id !== null && ids.has(item.source_card_id)).length;
}

export function actionCountLabel(count: number): string {
    return count === 1 ? "1 action" : `${count} actions`;
}

export function sourceCard(snapshot: BoardSnapshot, item: ActionItemView): CardView | null {
    if (item.source_card_id === null) {
        return null;
    }

    return snapshot.cards.find(card => card.id === item.source_card_id) ?? null;
}

export function revealedCardId(card: CardView): string {
    return card.parent_card_id ?? card.id;
}

export function nextStatus(status: ActionItemStatus): ActionItemStatus {
    return status === "open" ? "done" : "open";
}

export function statusFilterOptions(counts: ActionItemCountsView | undefined): StatusFilterOption[] {
    const count = (n: number | undefined) => (n === undefined ? "" : ` (${n})`);
    const total = counts === undefined ? undefined : counts.open + counts.done + counts.dropped;

    return [
        { value: "all", label: `All${count(total)}` },
        { value: "open", label: `Open${count(counts?.open)}` },
        { value: "done", label: `Done${count(counts?.done)}` },
        { value: "dropped", label: `Dropped${count(counts?.dropped)}` },
    ];
}

export function completionLabel(rate: number | null): string {
    if (rate === null) {
        return "Nothing to complete yet";
    }

    return `${Math.round(rate * 100)}% complete`;
}

export function assigneeChoices(items: readonly ActionItemListItem[], picked: PersonView | null): FilterChoice[] {
    const seen = new Map<string, string>();

    if (picked !== null) {
        seen.set(picked.user_id, picked.display_name);
    }

    for (const item of items) {
        if (item.assignee !== null) {
            seen.set(item.assignee.user_id, item.assignee.display_name);
        }
    }

    const people = [...seen]
        .map(([value, label]) => ({ value, label }))
        .toSorted((left, right) => left.label.localeCompare(right.label));

    return [...FIXED_ASSIGNEE_CHOICES, ...people];
}

export function teamChoices(teams: readonly TeamResponse[]): FilterChoice[] {
    const named = teams
        .map(team => ({ value: team.id, label: team.name }))
        .toSorted((left, right) => left.label.localeCompare(right.label));

    return [...FIXED_TEAM_CHOICES, ...named];
}

export function pickedPerson(items: readonly ActionItemListItem[], userId: string): PersonView | null {
    for (const item of items) {
        if (item.assignee?.user_id === userId) {
            return item.assignee;
        }
    }

    return null;
}

export function assigneeQuery(filter: AssigneeFilter): string | undefined {
    return filter === "anyone" ? undefined : filter;
}

export function teamQuery(filter: TeamFilter): string | undefined {
    return filter === "all" ? undefined : filter;
}

export function statusQuery(filter: StatusFilter): ActionItemStatus | undefined {
    return filter === "all" ? undefined : filter;
}
