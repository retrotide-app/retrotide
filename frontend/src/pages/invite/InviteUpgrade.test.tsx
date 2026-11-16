import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { InviteUpgrade } from "./InviteUpgrade";

describe("InviteUpgrade", () => {
    it("says the plan's limit and who to ask, without paid plans while pricing is hidden", () => {
        // given
        const upgrade = {
            error: "upgrade required",
            code: "upgrade_required",
            limit: "members" as const,
            allowed: 10,
            current: 10,
            plan: "free" as const,
        };

        // when
        renderWithProviders(<InviteUpgrade upgrade={upgrade} orgName="Harbour" />);

        // then
        const prompt = screen.getByRole("alert");
        expect(prompt).toHaveTextContent("The free plan allows 10 members.");
        expect(prompt).not.toHaveTextContent("Paid plans");
        expect(prompt).toHaveFocus();
        expect(screen.getByText(/has reached its plan's limit/)).toHaveTextContent(
            "Harbour has reached its plan's limit. Ask the person who invited you.",
        );
    });
});
