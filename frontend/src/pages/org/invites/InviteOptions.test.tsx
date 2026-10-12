import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeTeam } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AssignableOrgRole } from "../../../types/api";
import { InviteOptions } from "./InviteOptions";

const crew = makeTeam({ id: "team-crew", name: "Crew" });
const deck = makeTeam({ id: "team-deck", name: "Deck" });

function renderOptions(roles: readonly AssignableOrgRole[] = ["admin", "member"]) {
    const onRoleChange = vi.fn();
    const onTeamChange = vi.fn();

    renderWithProviders(
        <InviteOptions
            roles={roles}
            role="member"
            onRoleChange={onRoleChange}
            teams={[crew, deck]}
            teamId="team-deck"
            onTeamChange={onTeamChange}
        />,
    );

    return { onRoleChange, onTeamChange };
}

function optionNames(label: string): string[] {
    return within(screen.getByLabelText(label))
        .getAllByRole("option")
        .map(option => option.textContent ?? "");
}

describe("InviteOptions", () => {
    it("offers the roles the viewer may give, with the chosen one selected", () => {
        // when
        renderOptions();

        // then
        expect(optionNames("Role")).toEqual(["Admin", "Member"]);
        expect(screen.getByLabelText("Role")).toHaveValue("member");
    });

    it("offers only the member role when that is all the viewer may give", () => {
        // when
        renderOptions(["member"]);

        // then
        expect(optionNames("Role")).toEqual(["Member"]);
    });

    it("passes the role picked on", async () => {
        // given
        const user = userEvent.setup();
        const { onRoleChange } = renderOptions();

        // when
        await user.selectOptions(screen.getByLabelText("Role"), "admin");

        // then
        expect(onRoleChange).toHaveBeenCalledExactlyOnceWith("admin");
    });

    it("offers no team or one of the teams, says what choosing one does, and shows the one chosen", () => {
        // when
        renderOptions();

        // then
        expect(optionNames("Team")).toEqual(["No team", "Crew", "Deck"]);
        expect(screen.getByLabelText("Team")).toHaveValue("team-deck");
        expect(screen.getByLabelText("Team")).toHaveAccessibleDescription("They also join this team as a member.");
    });

    it("ties a refusal to the team field", () => {
        // when
        renderWithProviders(
            <InviteOptions
                roles={["member"]}
                role="member"
                onRoleChange={vi.fn()}
                teams={[crew]}
                teamId="team-crew"
                onTeamChange={vi.fn()}
                teamError="that team is archived"
            />,
        );

        // then
        expect(screen.getByLabelText("Team")).toBeInvalid();
        expect(screen.getByLabelText("Team")).toHaveAccessibleDescription(
            "They also join this team as a member. that team is archived",
        );
    });

    it("passes the team picked on", async () => {
        // given
        const user = userEvent.setup();
        const { onTeamChange } = renderOptions();

        // when
        await user.selectOptions(screen.getByLabelText("Team"), "team-crew");

        // then
        expect(onTeamChange).toHaveBeenCalledExactlyOnceWith("team-crew");
    });
});
