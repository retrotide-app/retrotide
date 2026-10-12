import type { Colour } from "../../types/api";

export const BOARD_TITLE_MAX_LENGTH = 200;

export const BOARD_CONTEXT_MAX_LENGTH = 500;

export const COLUMN_TITLE_MAX_LENGTH = 80;

export const COLUMN_DESCRIPTION_MAX_LENGTH = 200;

export const CARD_BODY_MAX_LENGTH = 2000;

export const COMMENT_BODY_MAX_LENGTH = 1000;

export const VOTE_LIMIT_MAX = 99;

const WHOLE_NUMBER = /^\d+$/;

export function voteLimitFrom(text: string): number | null {
    const trimmed = text.trim();

    if (!WHOLE_NUMBER.test(trimmed)) {
        return null;
    }

    const limit = Number(trimmed);

    return limit <= VOTE_LIMIT_MAX ? limit : null;
}

export const DEFAULT_COLUMN_COLOUR: Colour = "grey";

export const COLOUR_LABELS: Readonly<Record<Colour, string>> = {
    green: "Green",
    red: "Red",
    blue: "Blue",
    amber: "Amber",
    purple: "Purple",
    teal: "Teal",
    pink: "Pink",
    grey: "Grey",
};

export const BOARD_COLOURS: readonly Colour[] = ["green", "red", "blue", "amber", "purple", "teal", "pink", "grey"];
