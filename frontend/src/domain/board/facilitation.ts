import type { BoardSettingsView, BoardSnapshot, ParticipantView } from "../../types/api";
import { errorCode } from "../../utils/errorMessage";
import { boardInteractions } from "./capabilities";

export interface ParticipantRow {
    participant: ParticipantView;
    you: boolean;
    manageable: boolean;
}

export type LockName = "board" | "cards" | "moves";

export interface LockState {
    name: LockName;
    label: string;
    notice: string;
}

const FACILITATION_REFUSALS: Readonly<Record<string, string>> = {
    cannot_promote:
        "Only someone signed in through the team or organisation can be a co-facilitator. Guests and people who joined by link alone stay participants.",
    lead_facilitates:
        "This person facilitates through their team or organisation role, so they cannot be demoted or removed here.",
    remove_self: "You cannot remove yourself from the board.",
    timer_too_long: "A timer can run for an hour at most.",
    timer_not_running: "The timer is not running any more.",
    timer_not_paused: "The timer is not paused any more.",
    column_hidden: "That column is hidden from participants, so nothing in it can change until it is shown again.",
    column_revealed: "A card cannot move between a revealed column and one whose cards are still hidden.",
    column_vote_limit_reached: "You have used every vote this column allows.",
    card_not_shown: "Only a card everyone can see can be presented or highlighted.",
    invalid_column_vote_limit: "A column vote limit must be between 1 and 99, or none.",
};

const LOCKS: readonly LockState[] = [
    {
        name: "board",
        label: "Board locked",
        notice: "The board is locked. Everyone can read it, but only facilitators can change anything.",
    },
    {
        name: "cards",
        label: "Cards locked",
        notice: "Cards are locked, so participants cannot add, edit or delete cards.",
    },
    {
        name: "moves",
        label: "Moving locked",
        notice: "Moving cards is locked, so participants cannot move or merge cards.",
    },
];

function lockOn(settings: BoardSettingsView, name: LockName): boolean {
    switch (name) {
        case "board":
            return settings.board_locked;
        case "cards":
            return settings.cards_locked;
        case "moves":
            return settings.moves_locked;
    }
}

export function activeLocks(settings: BoardSettingsView): LockState[] {
    return LOCKS.filter(lock => lockOn(settings, lock.name));
}

export function facilitationRefusal(error: Error): string {
    const code = errorCode(error);

    if (code !== null && Object.hasOwn(FACILITATION_REFUSALS, code)) {
        return FACILITATION_REFUSALS[code];
    }

    return error.message;
}

function roleRank(participant: ParticipantView): number {
    return participant.role === "facilitator" ? 0 : 1;
}

export function onlineRows(rows: readonly ParticipantRow[], online: ReadonlySet<string> | null): ParticipantRow[] {
    return rows.filter(row => row.you || (online?.has(row.participant.participant_id) ?? false));
}

export function participantRows(snapshot: BoardSnapshot): ParticipantRow[] {
    const you = snapshot.viewer.participant_id;
    const manages = boardInteractions(snapshot).facilitate;

    return snapshot.participants
        .toSorted(
            (left, right) => roleRank(left) - roleRank(right) || left.display_name.localeCompare(right.display_name),
        )
        .map(participant => {
            const isYou = participant.participant_id === you;

            return { participant, you: isYou, manageable: manages && !isYou };
        });
}
