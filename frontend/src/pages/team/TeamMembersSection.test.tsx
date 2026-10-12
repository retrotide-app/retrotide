import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TeamViewer } from "../../domain/organisationRoles";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeOrganisationMember,
    makeQueryState,
    makeTeam,
    makeTeamMember,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { OrganisationMemberListResponse, TeamMemberListResponse, TeamMemberResponse } from "../../types/api";
import { formatDate } from "../../utils/time";
import { TeamMembersSection } from "./TeamMembersSection";

const {
    useTeamMembers,
    useOrganisationMembers,
    useChangeTeamMemberRole,
    useRemoveTeamMember,
    useAddTeamMember,
    useCreateInvite,
} = vi.hoisted(() => ({
    useTeamMembers: vi.fn(),
    useOrganisationMembers: vi.fn(),
    useChangeTeamMemberRole: vi.fn(),
    useRemoveTeamMember: vi.fn(),
    useAddTeamMember: vi.fn(),
    useCreateInvite: vi.fn(),
}));

vi.mock("../../hooks/queries/teams", () => ({ useTeamMembers }));
vi.mock("../../hooks/queries/organisations", () => ({ useOrganisationMembers }));
vi.mock("../../hooks/mutations/teams", () => ({ useChangeTeamMemberRole, useRemoveTeamMember, useAddTeamMember }));
vi.mock("../../hooks/mutations/invites", () => ({ useCreateInvite }));

const harbour = makeOrganisation({ id: "org-harbour", name: "Harbour" });
const crew = makeTeam({ id: "team-crew", org_id: "org-harbour", name: "Crew" });

const people = [
    makeOrganisationMember({ user_id: "user-sam", display_name: "Sam", email: "sam@example.test", role: "owner" }),
    makeOrganisationMember({ user_id: "user-lee", display_name: "Lee", email: "lee@example.test", role: "member" }),
    makeOrganisationMember({ user_id: "user-robin", display_name: "Robin", email: "robin@example.test" }),
    makeOrganisationMember({ user_id: "user-kit", display_name: "Kit", email: "kit@example.test", role: "member" }),
];

const lee = makeTeamMember({ user_id: "user-lee", display_name: "Lee", email: "lee@example.test", role: "lead" });
const robin = makeTeamMember({
    user_id: "user-robin",
    display_name: "Robin",
    email: "robin@example.test",
    role: "member",
    joined_at: "2026-09-03T10:00:00Z",
});

const owner: TeamViewer = { userId: "user-sam", orgRole: "owner", teamRole: "lead", siteRole: null };
const lead: TeamViewer = { userId: "user-lee", orgRole: "member", teamRole: "lead", siteRole: null };
const member: TeamViewer = { userId: "user-robin", orgRole: "member", teamRole: "member", siteRole: null };

function showTeamMembers(members: TeamMemberResponse[]) {
    useTeamMembers.mockReturnValue(makeQueryState<TeamMemberListResponse>({ data: { members } }));
}

function renderSection(viewer: TeamViewer = owner, managesMembers = true) {
    return renderWithProviders(
        <TeamMembersSection organisation={harbour} team={crew} viewer={viewer} managesMembers={managesMembers} />,
    );
}

function rowOf(name: string): HTMLElement {
    const row = within(screen.getByRole("list", { name: "Members of Crew" }))
        .getAllByRole("listitem")
        .find(candidate => within(candidate).queryByText(name) !== null);

    if (row === undefined) {
        throw new Error(`no row for ${name}`);
    }

    return row;
}

async function actionsFor(name: string): Promise<string[]> {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: `Actions for ${name}` }));

    const items = screen.getAllByRole("menuitem").map(item => item.textContent ?? "");
    await user.keyboard("{Escape}");

    return items;
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    vi.clearAllMocks();
    showTeamMembers([lee, robin]);
    useOrganisationMembers.mockReturnValue(
        makeQueryState<OrganisationMemberListResponse>({ data: { members: people } }),
    );
    useChangeTeamMemberRole.mockReturnValue(makeMutationState());
    useRemoveTeamMember.mockReturnValue(makeMutationState());
    useAddTeamMember.mockReturnValue(makeMutationState());
    useCreateInvite.mockReturnValue(makeMutationState());
});

describe("TeamMembersSection", () => {
    it("lists each person with their team role and when they joined, marks the viewer and counts them", () => {
        // when
        renderSection({ ...owner, userId: "user-lee" });

        // then
        expect(useTeamMembers).toHaveBeenCalledWith("team-crew");
        expect(screen.getByText("2 members")).toBeInTheDocument();
        expect(rowOf("Lee")).toHaveTextContent("Lee (you)");
        expect(rowOf("Lee")).toHaveTextContent("Lead");
        expect(rowOf("Robin")).toHaveTextContent("robin@example.test");
        expect(rowOf("Robin")).toHaveTextContent("Member");
        expect(within(rowOf("Robin")).getByText(formatDate("2026-09-03T10:00:00Z"))).toHaveAttribute(
            "dateTime",
            "2026-09-03T10:00:00Z",
        );
    });

    it("says it is loading while the team's members are on their way", () => {
        // given
        useTeamMembers.mockReturnValue(makeQueryState<TeamMemberListResponse>({ loading: true }));

        // when
        renderSection();

        // then
        expect(screen.getByText("Loading members...")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Add people" })).toBeDisabled();
    });

    it("says so when nobody is on the team", () => {
        // given
        showTeamMembers([]);

        // when
        renderSection();

        // then
        expect(screen.getByText("Nobody is on this team yet.")).toBeInTheDocument();
    });

    it("shows the server's reason when the team's members could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<TeamMemberListResponse>({
            error: makeApiError({ status: 500, message: "members unavailable" }),
        });
        useTeamMembers.mockReturnValue(failed);
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("members unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("opens the add people dialog from the section header", async () => {
        // given
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Add people" }));

        // then
        expect(screen.getByRole("dialog", { name: "Add people to Crew" })).toBeInTheDocument();
    });

    it("closes the dialog and says who was added once someone is added", async () => {
        // given
        const user = userEvent.setup();
        const addition = makeMutationState();
        addition.mutate.mockImplementation(succeed);
        useAddTeamMember.mockReturnValue(addition);
        renderSection();
        await user.click(screen.getByRole("button", { name: "Add people" }));

        // when
        await user.selectOptions(screen.getByLabelText("Person"), "user-kit");
        await user.click(screen.getByRole("button", { name: "Add to team" }));

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Kit added to Crew.");
    });
});

describe("TeamMembersSection actions each viewer may take", () => {
    it("offers the owner every change on the people on the team", async () => {
        // when
        renderSection(owner);

        // then
        expect(await actionsFor("Robin")).toEqual(["Make lead", "Remove"]);
        expect(await actionsFor("Lee")).toEqual(["Make member", "Remove"]);
    });

    it("lets a lead remove a member but neither make a lead nor act on another lead or themselves", async () => {
        // given
        const other = makeTeamMember({ user_id: "user-kit", display_name: "Kit", role: "lead" });
        showTeamMembers([lee, robin, other]);

        // when
        renderSection(lead);

        // then
        expect(await actionsFor("Robin")).toEqual(["Remove"]);
        expect(within(rowOf("Kit")).queryByRole("button")).not.toBeInTheDocument();
        expect(within(rowOf("Lee")).queryByRole("button")).not.toBeInTheDocument();
    });

    it("offers a team member no actions and no way to add people", () => {
        // when
        renderSection(member, false);

        // then
        expect(within(screen.getByRole("list", { name: "Members of Crew" })).queryByRole("button")).toBeNull();
        expect(useOrganisationMembers).not.toHaveBeenCalled();
        expect(screen.queryByRole("button", { name: "Add people" })).not.toBeInTheDocument();
    });

    it("keeps a lead from acting on an organisation admin on the team, who leads every team", async () => {
        // given
        const alex = makeTeamMember({ user_id: "user-alex", display_name: "Alex", role: "member", org_role: "admin" });
        showTeamMembers([lee, robin, alex]);

        // when
        renderSection(lead);

        // then
        expect(within(rowOf("Alex")).queryByRole("button")).not.toBeInTheDocument();
        expect(await actionsFor("Robin")).toEqual(["Remove"]);
    });

    it("hides every action on someone who is site staff, even from the owner", () => {
        // given
        const staff = makeTeamMember({ ...robin, site_role: "support" });
        showTeamMembers([lee, staff]);

        // when
        renderSection(owner);

        // then
        expect(within(rowOf("Robin")).queryByRole("button")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Actions for Lee" })).toBeInTheDocument();
    });

    it("lets a lead who is site staff make a member a lead", async () => {
        // when
        renderSection({ ...lead, siteRole: "support" });

        // then
        expect(await actionsFor("Robin")).toContain("Make lead");
    });
});

describe("TeamMembersSection changing a team role", () => {
    it("changes the person's team role from their row menu", async () => {
        // given
        const user = userEvent.setup();
        const changeRole = makeMutationState();
        useChangeTeamMemberRole.mockReturnValue(changeRole);
        renderSection(owner);

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Robin" }));
        await user.click(screen.getByRole("menuitem", { name: "Make lead" }));

        // then
        expect(changeRole.mutate).toHaveBeenCalledExactlyOnceWith({
            teamId: "team-crew",
            userId: "user-robin",
            role: "lead",
        });
    });

    it("says what the person's team role now is", () => {
        // given
        useChangeTeamMemberRole.mockReturnValue(
            makeMutationState({
                isSuccess: true,
                variables: { teamId: "team-crew", userId: "user-robin", role: "lead" },
            }),
        );

        // when
        renderSection(owner);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Robin is now a lead.");
    });

    it("shows the server's refusal and moves focus to it", () => {
        // given
        useChangeTeamMemberRole.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 403, message: "not allowed" }) }),
        );

        // when
        renderSection(owner);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("not allowed");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});

describe("TeamMembersSection removing someone from the team", () => {
    it("asks before removing the person, removes them and says so", async () => {
        // given
        const user = userEvent.setup();
        const removal = makeMutationState();
        removal.mutate.mockImplementation(succeed);
        useRemoveTeamMember.mockReturnValue(removal);
        const { rerender } = renderSection(owner);

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Robin" }));
        await user.click(screen.getByRole("menuitem", { name: "Remove" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "Remove Robin from Crew?" });
        expect(dialog).toHaveTextContent("Robin leaves Crew but stays in Harbour.");

        // when
        await user.click(within(dialog).getByRole("button", { name: "Remove" }));
        useRemoveTeamMember.mockReturnValue(makeMutationState({ isSuccess: true }));
        rerender(<TeamMembersSection organisation={harbour} team={crew} viewer={owner} managesMembers />);

        // then
        expect(removal.mutate).toHaveBeenCalledWith(
            { teamId: "team-crew", userId: "user-robin" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Robin removed from Crew.");
    });

    it("keeps the person on the team when the viewer changes their mind", async () => {
        // given
        const user = userEvent.setup();
        const removal = makeMutationState();
        useRemoveTeamMember.mockReturnValue(removal);
        renderSection(owner);
        await user.click(screen.getByRole("button", { name: "Actions for Robin" }));
        await user.click(screen.getByRole("menuitem", { name: "Remove" }));

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(removal.mutate).not.toHaveBeenCalled();
    });

    it("keeps the check open with the server's reason when the removal fails", async () => {
        // given
        const user = userEvent.setup();
        useRemoveTeamMember.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 409, message: "cannot remove" }) }),
        );
        renderSection(owner);

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Robin" }));
        await user.click(screen.getByRole("menuitem", { name: "Remove" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "Remove Robin from Crew?" });
        expect(within(dialog).getByRole("alert")).toHaveTextContent("cannot remove");
    });
});
