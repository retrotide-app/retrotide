import { errorCode } from "../utils/errorMessage";

export type RefusalCode =
    | "org_name_required"
    | "org_name_too_long"
    | "team_name_required"
    | "team_name_too_long"
    | "team_name_taken"
    | "slug_invalid"
    | "slug_reserved"
    | "slug_taken"
    | "team_archived"
    | "new_owner_limit_reached"
    | "email_taken"
    | "email_unavailable"
    | "display_name_required"
    | "board_title_required"
    | "board_title_too_long"
    | "board_context_too_long"
    | "unknown_template"
    | "invalid_anonymity"
    | "column_title_required"
    | "column_title_too_long"
    | "column_description_too_long"
    | "invalid_colour"
    | "card_body_required"
    | "card_body_too_long"
    | "board_password_blank"
    | "board_password_too_long"
    | "invalid_link_access"
    | "action_item_title_required"
    | "action_item_title_too_long"
    | "too_many_mentions"
    | "invalid_due_date"
    | "not_board_member"
    | "members_only"
    | "archived";

export const ORGANISATION_NAME_REFUSALS: readonly RefusalCode[] = ["org_name_required", "org_name_too_long"];

export const TEAM_NAME_REFUSALS: readonly RefusalCode[] = [
    "team_name_required",
    "team_name_too_long",
    "team_name_taken",
];

export const ADDRESS_REFUSALS: readonly RefusalCode[] = ["slug_invalid", "slug_reserved", "slug_taken"];

export const INVITE_TEAM_REFUSALS: readonly RefusalCode[] = ["team_archived"];

export const DISPLAY_NAME_REFUSALS: readonly RefusalCode[] = ["display_name_required"];

export const EMAIL_REFUSALS: readonly RefusalCode[] = ["email_taken"];

export const EMAIL_DELIVERY_REFUSALS: readonly RefusalCode[] = ["email_unavailable"];

export const EMAIL_UNAVAILABLE = "This site cannot send email yet, so nothing was sent.";

export const BOARD_TITLE_REFUSALS: readonly RefusalCode[] = ["board_title_required", "board_title_too_long"];

export const BOARD_CONTEXT_REFUSALS: readonly RefusalCode[] = ["board_context_too_long"];

export const BOARD_TEMPLATE_REFUSALS: readonly RefusalCode[] = ["unknown_template"];

export const BOARD_ANONYMITY_REFUSALS: readonly RefusalCode[] = ["invalid_anonymity"];

export const COLUMN_TITLE_REFUSALS: readonly RefusalCode[] = ["column_title_required", "column_title_too_long"];

export const COLUMN_DESCRIPTION_REFUSALS: readonly RefusalCode[] = ["column_description_too_long"];

export const COLOUR_REFUSALS: readonly RefusalCode[] = ["invalid_colour"];

export const CARD_BODY_REFUSALS: readonly RefusalCode[] = ["card_body_required", "card_body_too_long"];

export const BOARD_PASSWORD_REFUSALS: readonly RefusalCode[] = ["board_password_blank", "board_password_too_long"];

export const ACTION_ITEM_TITLE_REFUSALS: readonly RefusalCode[] = [
    "action_item_title_required",
    "action_item_title_too_long",
    "too_many_mentions",
];

export const DUE_DATE_REFUSALS: readonly RefusalCode[] = ["invalid_due_date"];

export const NAMED_PERSON_REFUSALS: readonly RefusalCode[] = ["not_board_member", "members_only"];

export const MEMBERS_ONLY_REFUSALS: readonly RefusalCode[] = ["members_only"];

export const MENTION_REFUSALS: readonly RefusalCode[] = ["too_many_mentions", "not_board_member", "members_only"];

export function refusedWith(error: Error | null, codes: readonly RefusalCode[]): boolean {
    const code = errorCode(error);

    return code !== null && codes.some(refusal => refusal === code);
}

export function emailDeliveryRefusal(error: Error | null): string | null {
    if (!refusedWith(error, EMAIL_DELIVERY_REFUSALS)) {
        return null;
    }

    return EMAIL_UNAVAILABLE;
}

export function fieldRefusal(error: Error | null, codes: readonly RefusalCode[]): string | null {
    if (error === null || !refusedWith(error, codes)) {
        return null;
    }

    return error.message;
}
