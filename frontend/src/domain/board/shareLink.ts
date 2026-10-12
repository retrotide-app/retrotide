import type {
    BoardSnapshot,
    JoinBoardRequest,
    JoinBoardResponse,
    LinkAccess,
    OrganisationResponse,
} from "../../types/api";
import { errorCode, errorStatus } from "../../utils/errorMessage";
import { isReadOnly } from "./capabilities";
import { boardPath, joinBoardPath, sharedBoardPath } from "./paths";

export interface LinkAccessOption {
    value: LinkAccess;
    label: string;
    description: string;
}

export interface JoinDraft {
    name: string;
    anonymous: boolean;
    password: string;
}

export interface JoinProblems {
    name?: string;
    password?: string;
}

export interface RejoinState {
    joinSlug: string;
}

export const GUEST_NAME_MAX_LENGTH = 40;

export const LINK_ACCESS_OPTIONS: readonly LinkAccessOption[] = [
    {
        value: "guests",
        label: "Anyone with the link",
        description: "People join with their name, or as Anonymous, without an account.",
    },
    {
        value: "accounts",
        label: "Signed in only",
        description: "People sign in before they join. Guests already on the board lose access.",
    },
    {
        value: "off",
        label: "Off",
        description: "Nobody can join with the link, and everyone who joined with it loses access.",
    },
];

const UNAUTHORISED_STATUS = 401;

const NOT_FOUND_STATUS = 404;

const TOO_MANY_REQUESTS_STATUS = 429;

const SIGN_IN_REQUIRED = "sign_in_required";

const ACCOUNT_CODES: readonly string[] = ["account_required", "guests_not_allowed"];

const JOIN_REFUSALS: Readonly<Record<string, string>> = {
    account_required: "This board is only for people with an account. Sign in to join it.",
    guests_not_allowed: "This organisation does not let guests join its boards. Sign in to join it.",
    link_disabled: "The link to this board is switched off. Ask whoever shared it to switch it back on.",
    removed: "A facilitator removed you from this board, so you cannot join it again.",
    archived: "This board is archived, so nobody can join it.",
    board_full: "This board already holds as many people as it can.",
    password_attempts: "Too many wrong passwords for this board. Wait a few minutes, then try again.",
    board_busy: "The board is busy. Try again in a moment.",
};

const PASSWORD_REFUSALS: Readonly<Record<string, string>> = {
    password_required: "This board needs its password.",
    wrong_password: "That is not the board's password.",
};

const SHARED_BOARD_REFUSALS: Readonly<Record<string, string>> = {
    password_required: "This board's password changed. Open the board's link again and enter the new one.",
    account_required: "This board is only for people with an account now. Sign in to see it.",
    guests_not_allowed: "This organisation no longer lets guests join its boards. Sign in to see it.",
    link_disabled: "The link to this board has been switched off.",
    removed: "A facilitator removed you from this board.",
};

const LINK_NOT_FOUND =
    "This link does not lead to a board. The board may have been deleted, or its link changed. Ask whoever shared it for the current link.";

const TOO_MANY_JOINS = "Too many attempts to join. Wait a minute, then try again.";

const BOARD_NOT_FOUND = "This board is not here. It may have been deleted, or its link changed.";

const PRIVATE_SITE = "This site is private. Sign in, or open the board's link again to join it.";

const GUEST_PASS_GONE =
    "This browser no longer holds its guest pass for the board. Open the board's link again to rejoin.";

const SIGN_IN_TO_OPEN = "Sign in, or open the board's link, to see this board.";

const NAME_MISSING = "Enter your name, or choose to join as Anonymous.";

const PASSWORD_MISSING = "Enter the board's password.";

function lookup(table: Readonly<Record<string, string>>, code: string | null): string | null {
    if (code === null || !Object.hasOwn(table, code)) {
        return null;
    }

    return table[code];
}

export function shareLinkUrl(origin: string, slug: string): string {
    return `${origin}${joinBoardPath(slug)}`;
}

export function canShareBoard({ board, viewer }: BoardSnapshot): boolean {
    return !isReadOnly(board) && viewer.capabilities.includes("board.manage");
}

export function joinProblems(draft: JoinDraft, asGuest: boolean, requiresPassword: boolean): JoinProblems {
    const problems: JoinProblems = {};

    if (asGuest && !draft.anonymous && draft.name.trim() === "") {
        problems.name = NAME_MISSING;
    }

    if (requiresPassword && draft.password === "") {
        problems.password = PASSWORD_MISSING;
    }

    return problems;
}

export function joinRequest(draft: JoinDraft, asGuest: boolean, requiresPassword: boolean): JoinBoardRequest {
    const request: JoinBoardRequest = {};
    const name = draft.name.trim();

    if (asGuest && !draft.anonymous && name !== "") {
        request.display_name = name;
    }

    if (requiresPassword && draft.password !== "") {
        request.password = draft.password;
    }

    return request;
}

export function joinRefusal(error: Error | null): string | null {
    if (error === null) {
        return null;
    }

    const known = lookup(JOIN_REFUSALS, errorCode(error));

    if (known !== null) {
        return known;
    }

    const status = errorStatus(error);

    if (status === NOT_FOUND_STATUS) {
        return LINK_NOT_FOUND;
    }

    if (status === TOO_MANY_REQUESTS_STATUS) {
        return TOO_MANY_JOINS;
    }

    return null;
}

export function joinPasswordRefusal(error: Error | null): string | null {
    const code = errorCode(error);

    return lookup(PASSWORD_REFUSALS, code);
}

export function needsAccount(error: Error | null): boolean {
    const code = errorCode(error);

    return code !== null && ACCOUNT_CODES.includes(code);
}

export function sharedBoardRefusal(error: Error | null, canRejoin: boolean): string | null {
    if (error === null) {
        return null;
    }

    const code = errorCode(error);
    const status = errorStatus(error);

    if (status === UNAUTHORISED_STATUS && code === SIGN_IN_REQUIRED) {
        return PRIVATE_SITE;
    }

    if (status === UNAUTHORISED_STATUS) {
        return canRejoin ? GUEST_PASS_GONE : SIGN_IN_TO_OPEN;
    }

    if (status === NOT_FOUND_STATUS) {
        return BOARD_NOT_FOUND;
    }

    return lookup(SHARED_BOARD_REFUSALS, code);
}

export function joinedBoardPath(
    joined: JoinBoardResponse,
    organisations: readonly Pick<OrganisationResponse, "id" | "slug">[],
): string {
    const home = organisations.find(organisation => organisation.id === joined.org_id);

    return home === undefined ? sharedBoardPath(joined.board_id) : boardPath(home.slug, joined.board_id);
}

export function rejoinState(slug: string): RejoinState {
    return { joinSlug: slug };
}

export function rejoinSlugFrom(state: unknown): string | null {
    if (state === null || typeof state !== "object") {
        return null;
    }

    const slug = (state as Partial<Record<keyof RejoinState, unknown>>).joinSlug;

    if (typeof slug !== "string" || slug === "") {
        return null;
    }

    return slug;
}
