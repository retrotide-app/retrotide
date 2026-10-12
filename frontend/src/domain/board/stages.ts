import type { BoardSettingsView, BoardStage, BoardView } from "../../types/api";

export interface StageInfo {
    label: string;
    purpose: string;
}

export interface StageStep {
    to: BoardStage;
    label: string;
    revealsCards: boolean;
    endsRetro: boolean;
}

export const BOARD_STAGES: readonly BoardStage[] = ["write", "vote", "discuss", "actions", "end"];

export const STAGE_INFO: Readonly<Record<BoardStage, StageInfo>> = {
    write: {
        label: "Write",
        purpose: "Write your cards. Nobody can vote or comment yet.",
    },
    vote: {
        label: "Vote",
        purpose: "The cards are revealed. Vote for the ones you most want to talk about.",
    },
    discuss: {
        label: "Discuss",
        purpose: "Vote totals are shown. Talk the cards through, comment on them and merge any that match.",
    },
    actions: {
        label: "Actions",
        purpose: "Agree what to do next and add action items. The cards can no longer be changed.",
    },
    end: {
        label: "End",
        purpose: "The retro has ended, so the board can only be read.",
    },
};

export const STAGE_LABELS: readonly string[] = BOARD_STAGES.map(stage => STAGE_INFO[stage].label);

export function nextStage(stage: BoardStage): BoardStage | null {
    const index = BOARD_STAGES.indexOf(stage);

    if (index < 0 || index + 1 >= BOARD_STAGES.length) {
        return null;
    }

    return BOARD_STAGES[index + 1];
}

export function nextStep(board: BoardView): StageStep | null {
    const to = nextStage(board.stage);

    if (to === null) {
        return null;
    }

    return {
        to,
        label: STAGE_INFO[to].label,
        revealsCards: to === "vote" && board.cards_hidden,
        endsRetro: to === "end",
    };
}

export function previousStage(stage: BoardStage): BoardStage | null {
    const index = BOARD_STAGES.indexOf(stage);

    if (index <= 0 || stage === "end") {
        return null;
    }

    return BOARD_STAGES[index - 1];
}

export function backStep(board: BoardView): BoardStage | null {
    if (board.closed) {
        return null;
    }

    return previousStage(board.stage);
}

export function totalsHidden(settings: BoardSettingsView, stage: BoardStage): boolean {
    return settings.vote_counts_hidden && (stage === "write" || stage === "vote");
}
