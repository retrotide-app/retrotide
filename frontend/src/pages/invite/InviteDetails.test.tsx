import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeInviteDetails } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { InviteDetails } from "./InviteDetails";

function paragraph(text: string | RegExp) {
    return screen.getByText((_content, element) => {
        if (element?.tagName !== "P") {
            return false;
        }

        const shown = element.textContent ?? "";

        return typeof text === "string" ? shown === text : text.test(shown);
    });
}

describe("InviteDetails", () => {
    it("names the person who sent the invite, the organisation and the role", () => {
        // given
        const invite = makeInviteDetails({ inviter_name: "Sam", org_name: "Harbour", role: "member" });

        // when
        renderWithProviders(<InviteDetails invite={invite} />);

        // then
        expect(paragraph("Sam invited you to join Harbour as a member.")).toBeInTheDocument();
    });

    it("says who is invited without a name when the invite carries none", () => {
        // given
        const invite = makeInviteDetails({ inviter_name: undefined, role: "admin" });

        // when
        renderWithProviders(<InviteDetails invite={invite} />);

        // then
        expect(paragraph("You are invited to join Harbour as an admin.")).toBeInTheDocument();
    });

    it("names the team the invite also joins", () => {
        // given
        const invite = makeInviteDetails({ team_name: "Crew" });

        // when
        renderWithProviders(<InviteDetails invite={invite} />);

        // then
        expect(paragraph("You will also join the team Crew.")).toBeInTheDocument();
    });

    it("mentions no team when the invite joins the organisation only", () => {
        // given
        const invite = makeInviteDetails({ team_name: undefined });

        // when
        renderWithProviders(<InviteDetails invite={invite} />);

        // then
        expect(screen.queryByText(/also join the team/)).not.toBeInTheDocument();
    });

    it("says when the invite expires", () => {
        // given
        const invite = makeInviteDetails({ expires_at: "2026-10-03T12:00:00Z" });

        // when
        renderWithProviders(<InviteDetails invite={invite} />);

        // then
        expect(paragraph(/^The invite expires on 3 October 2026 at \d{2}:\d{2}\.$/)).toBeInTheDocument();
    });

    it("leaves the expiry out when the server sends a date it cannot read", () => {
        // given
        const invite = makeInviteDetails({ expires_at: "" });

        // when
        renderWithProviders(<InviteDetails invite={invite} />);

        // then
        expect(screen.queryByText(/expires/)).not.toBeInTheDocument();
    });
});
