import { describe, expect, it } from "vitest";
import {
    ACTION_ITEM_TITLE_REFUSALS,
    ADDRESS_REFUSALS,
    BOARD_ANONYMITY_REFUSALS,
    BOARD_CONTEXT_REFUSALS,
    BOARD_TEMPLATE_REFUSALS,
    BOARD_TITLE_REFUSALS,
    CARD_BODY_REFUSALS,
    COLOUR_REFUSALS,
    COLUMN_DESCRIPTION_REFUSALS,
    COLUMN_TITLE_REFUSALS,
    DISPLAY_NAME_REFUSALS,
    DUE_DATE_REFUSALS,
    EMAIL_DELIVERY_REFUSALS,
    EMAIL_UNAVAILABLE,
    emailDeliveryRefusal,
    fieldRefusal,
    INVITE_TEAM_REFUSALS,
    MEMBERS_ONLY_REFUSALS,
    NAMED_PERSON_REFUSALS,
    ORGANISATION_NAME_REFUSALS,
    refusedWith,
    TEAM_NAME_REFUSALS,
} from "./refusals";

interface Refusal {
    status: number;
    message: string;
    code?: string;
}

function makeApiError({ status, message, code }: Refusal): Error {
    return Object.assign(new Error(message), { name: "ApiError", status, code: code ?? null });
}

describe("fieldRefusal", () => {
    it("gives the server's text for a refusal whose code the field owns", () => {
        // given
        const error = makeApiError({ status: 409, message: "that address is already taken", code: "slug_taken" });

        // when
        const refusal = fieldRefusal(error, ADDRESS_REFUSALS);

        // then
        expect(refusal).toBe("that address is already taken");
    });

    it("gives nothing for a refusal another field owns, one with no code, or no refusal at all", () => {
        // given
        const nameTooLong = makeApiError({
            status: 400,
            message: "an organisation name can be at most 80 characters",
            code: "org_name_too_long",
        });
        const uncoded = makeApiError({ status: 409, message: "the team still has boards" });

        // when
        const refusals = [
            fieldRefusal(nameTooLong, ADDRESS_REFUSALS),
            fieldRefusal(uncoded, TEAM_NAME_REFUSALS),
            fieldRefusal(null, ORGANISATION_NAME_REFUSALS),
        ];

        // then
        expect(refusals).toEqual([null, null, null]);
    });

    it("ties each named code to its field", () => {
        // when
        const owners = {
            organisationName: ORGANISATION_NAME_REFUSALS,
            teamName: TEAM_NAME_REFUSALS,
            address: ADDRESS_REFUSALS,
            inviteTeam: INVITE_TEAM_REFUSALS,
            displayName: DISPLAY_NAME_REFUSALS,
        };

        // then
        expect(owners).toEqual({
            organisationName: ["org_name_required", "org_name_too_long"],
            teamName: ["team_name_required", "team_name_too_long", "team_name_taken"],
            address: ["slug_invalid", "slug_reserved", "slug_taken"],
            inviteTeam: ["team_archived"],
            displayName: ["display_name_required"],
        });
    });
});

describe("refusedWith", () => {
    it("tells a refusal that carries one of the codes from any other", () => {
        // given
        const taken = makeApiError({ status: 409, message: "taken", code: "email_taken" });
        const other = makeApiError({ status: 409, message: "pending", code: "invite_pending" });

        // when
        const matched = [refusedWith(taken, ["email_taken"]), refusedWith(other, ["email_taken"])];

        // then
        expect(matched).toEqual([true, false]);
    });
});

describe("board refusals", () => {
    it("ties each board code to the field that shows it", () => {
        // when
        const owners = {
            boardTitle: BOARD_TITLE_REFUSALS,
            boardContext: BOARD_CONTEXT_REFUSALS,
            boardTemplate: BOARD_TEMPLATE_REFUSALS,
            boardAnonymity: BOARD_ANONYMITY_REFUSALS,
            columnTitle: COLUMN_TITLE_REFUSALS,
            columnDescription: COLUMN_DESCRIPTION_REFUSALS,
            colour: COLOUR_REFUSALS,
            cardBody: CARD_BODY_REFUSALS,
        };

        // then
        expect(owners).toEqual({
            boardTitle: ["board_title_required", "board_title_too_long"],
            boardContext: ["board_context_too_long"],
            boardTemplate: ["unknown_template"],
            boardAnonymity: ["invalid_anonymity"],
            columnTitle: ["column_title_required", "column_title_too_long"],
            columnDescription: ["column_description_too_long"],
            colour: ["invalid_colour"],
            cardBody: ["card_body_required", "card_body_too_long"],
        });
    });

    it("gives the server's text for a card refusal on the card's text field only", () => {
        // given
        const tooLong = makeApiError({
            status: 400,
            message: "a card can be at most 2000 characters",
            code: "card_body_too_long",
        });

        // when
        const refusals = [fieldRefusal(tooLong, CARD_BODY_REFUSALS), fieldRefusal(tooLong, COLUMN_TITLE_REFUSALS)];

        // then
        expect(refusals).toEqual(["a card can be at most 2000 characters", null]);
    });
});

describe("action item refusals", () => {
    it("ties each action item code to the field that shows it", () => {
        // when
        const owners = {
            title: ACTION_ITEM_TITLE_REFUSALS,
            dueDate: DUE_DATE_REFUSALS,
            namedPerson: NAMED_PERSON_REFUSALS,
            membersOnly: MEMBERS_ONLY_REFUSALS,
        };

        // then
        expect(owners).toEqual({
            title: ["action_item_title_required", "action_item_title_too_long", "too_many_mentions"],
            dueDate: ["invalid_due_date"],
            namedPerson: ["not_board_member", "members_only"],
            membersOnly: ["members_only"],
        });
    });

    it("shows a person who cannot open the board on the assignee field, not the title", () => {
        // given
        const refused = makeApiError({
            status: 400,
            message: "that person cannot open this board",
            code: "not_board_member",
        });

        // when
        const refusals = [
            fieldRefusal(refused, NAMED_PERSON_REFUSALS),
            fieldRefusal(refused, ACTION_ITEM_TITLE_REFUSALS),
        ];

        // then
        expect(refusals).toEqual(["that person cannot open this board", null]);
    });
});

describe("emailDeliveryRefusal", () => {
    it("says plainly that nothing was sent when the site has no email provider", () => {
        // given
        const unavailable = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });

        // when
        const refusal = emailDeliveryRefusal(unavailable);

        // then
        expect(refusal).toBe(EMAIL_UNAVAILABLE);
        expect(EMAIL_DELIVERY_REFUSALS).toEqual(["email_unavailable"]);
    });

    it("gives nothing for any other failure or none", () => {
        // given
        const taken = makeApiError({ status: 409, message: "taken", code: "email_taken" });
        const uncoded = makeApiError({ status: 503, message: "the service is down" });

        // when
        const refusals = [emailDeliveryRefusal(taken), emailDeliveryRefusal(uncoded), emailDeliveryRefusal(null)];

        // then
        expect(refusals).toEqual([null, null, null]);
    });
});
