import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeOrganisation, makeQueryState, makeTeam, makeTeamMember } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { OrgRole, TeamMemberListResponse, TeamResponse } from "../../../types/api";
import { TeamCard } from "./TeamCard";

const { useTeamMembers } = vi.hoisted(() => ({ useTeamMembers: vi.fn() }));

vi.mock("../../../hooks/queries/teams", () => ({ useTeamMembers }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour" });
const crew = makeTeam({ id: "team-crew", name: "Crew", role: "lead" });

function renderCard(team: TeamResponse = crew, orgRole: OrgRole | null = "member", boardCount: number | null = 3) {
    return renderWithProviders(
        <TeamCard organisation={harbour} team={team} orgRole={orgRole} boardCount={boardCount} />,
    );
}

function factOf(term: string): string {
    const card = screen.getByRole("article");
    const label = within(card).getByText(term);

    return label.nextElementSibling?.textContent ?? "";
}

beforeEach(() => {
    vi.clearAllMocks();
    useTeamMembers.mockReturnValue(
        makeQueryState<TeamMemberListResponse>({
            data: {
                members: [makeTeamMember({ user_id: "user-lee" }), makeTeamMember({ user_id: "user-robin" })],
            },
        }),
    );
});

describe("TeamCard", () => {
    it("names the team and shows the viewer's role, its members and its boards", () => {
        // when
        renderCard();

        // then
        expect(screen.getByRole("article", { name: "Crew" })).toBeInTheDocument();
        expect(useTeamMembers).toHaveBeenCalledWith("team-crew");
        expect(factOf("Your role")).toBe("Lead");
        expect(factOf("Members")).toBe("2 members");
        expect(factOf("Boards")).toBe("3 boards");
    });

    it("opens the team's page", () => {
        // when
        renderCard();

        // then
        expect(screen.getByRole("link", { name: "Open" })).toHaveAttribute("href", "/o/harbour/teams/team-crew");
        expect(screen.getByRole("link", { name: "Open" })).toHaveAccessibleDescription("Crew");
    });

    it("treats an owner as a lead of a team they were never added to", () => {
        // when
        renderCard(makeTeam({ ...crew, role: undefined }), "owner");

        // then
        expect(factOf("Your role")).toBe("Lead");
        expect(screen.getByRole("link", { name: "Open" })).toBeInTheDocument();
    });

    it("shows only the name and role of a team the viewer is not on, without reading its members", () => {
        // when
        renderCard(makeTeam({ ...crew, role: undefined }), "member");

        // then
        expect(factOf("Your role")).toBe("Not on this team");
        expect(useTeamMembers).toHaveBeenCalledWith("");
        expect(screen.queryByText("Boards")).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Open" })).not.toBeInTheDocument();
    });

    it("leaves the board count out until it is known", () => {
        // when
        renderCard(crew, "member", null);

        // then
        expect(screen.queryByText("Boards")).not.toBeInTheDocument();
    });

    it("tags an archived team", () => {
        // when
        renderCard(makeTeam({ ...crew, archived_at: "2026-09-10T09:00:00Z" }));

        // then
        expect(screen.getByText("Archived")).toBeInTheDocument();
    });

    it("shows the server's reason when the members could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<TeamMemberListResponse>({
            error: makeApiError({ status: 500, message: "members unavailable" }),
        });
        useTeamMembers.mockReturnValue(failed);
        renderCard();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("members unavailable");
        expect(screen.queryByText("Members")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
