import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TeamViewer } from "../../domain/organisationRoles";
import {
    makeApiError,
    makeCreatedInvite,
    makeMutationState,
    makeOrganisation,
    makeOrganisationMember,
    makeQueryState,
    makeTeam,
    makeTeamMember,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { OrganisationMemberListResponse, OrganisationMemberResponse, TeamMemberResponse } from "../../types/api";
import { AddPeopleDialog } from "./AddPeopleDialog";

const { useOrganisationMembers, useAddTeamMember, useCreateInvite, copyText } = vi.hoisted(() => ({
    useOrganisationMembers: vi.fn(),
    useAddTeamMember: vi.fn(),
    useCreateInvite: vi.fn(),
    copyText: vi.fn(),
}));

vi.mock("../../hooks/queries/organisations", () => ({ useOrganisationMembers }));
vi.mock("../../hooks/mutations/teams", () => ({ useAddTeamMember }));
vi.mock("../../hooks/mutations/invites", () => ({ useCreateInvite }));
vi.mock("../../platform/clipboard", () => ({ copyText }));

const harbour = makeOrganisation({ id: "org-harbour", name: "Harbour" });
const crew = makeTeam({ id: "team-crew", org_id: "org-harbour", name: "Crew" });

const sam = makeOrganisationMember({ user_id: "user-sam", display_name: "Sam", role: "owner" });
const lee = makeOrganisationMember({ user_id: "user-lee", display_name: "Lee", email: "lee@example.test" });
const kit = makeOrganisationMember({ user_id: "user-kit", display_name: "Kit", email: "kit@example.test" });
const alex = makeOrganisationMember({
    user_id: "user-alex",
    display_name: "Alex",
    email: "alex@example.test",
    role: "admin",
});

const leeOnTeam = makeTeamMember({ user_id: "user-lee", display_name: "Lee", role: "lead" });

const owner: TeamViewer = { userId: "user-sam", orgRole: "owner", teamRole: "lead", siteRole: null };
const lead: TeamViewer = { userId: "user-lee", orgRole: "member", teamRole: "lead", siteRole: null };

interface DialogOptions {
    viewer?: TeamViewer;
    people?: OrganisationMemberResponse[];
    members?: TeamMemberResponse[];
}

function renderDialog({ viewer = owner, people = [sam, lee, kit, alex], members = [leeOnTeam] }: DialogOptions = {}) {
    const onDone = vi.fn();
    const onClose = vi.fn();
    useOrganisationMembers.mockReturnValue(
        makeQueryState<OrganisationMemberListResponse>({ data: { members: people } }),
    );

    renderWithProviders(
        <AddPeopleDialog
            open
            organisation={harbour}
            team={crew}
            viewer={viewer}
            members={members}
            onDone={onDone}
            onClose={onClose}
        />,
    );

    return { onDone, onClose };
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

function personOptions(): string[] {
    return within(screen.getByLabelText("Person"))
        .getAllByRole("option")
        .map(option => option.textContent ?? "");
}

beforeEach(() => {
    vi.clearAllMocks();
    useAddTeamMember.mockReturnValue(makeMutationState());
    useCreateInvite.mockReturnValue(makeMutationState());
    copyText.mockResolvedValue(undefined);
});

describe("AddPeopleDialog adding someone already in the organisation", () => {
    it("lists the people who are not on the team yet, leaving out the viewer", () => {
        // when
        renderDialog();

        // then
        expect(screen.getByRole("dialog", { name: "Add people to Crew" })).toBeInTheDocument();
        expect(useOrganisationMembers).toHaveBeenCalledWith("org-harbour");
        expect(personOptions()).toEqual(["Choose a person", "Kit (kit@example.test)", "Alex (alex@example.test)"]);
    });

    it("adds the chosen person with the chosen team role and reports it", async () => {
        // given
        const user = userEvent.setup();
        const addition = makeMutationState();
        addition.mutate.mockImplementation(succeed);
        useAddTeamMember.mockReturnValue(addition);
        const { onDone } = renderDialog();

        // when
        await user.selectOptions(screen.getByLabelText("Person"), "user-kit");
        await user.selectOptions(screen.getByLabelText("Team role"), "lead");
        await user.click(screen.getByRole("button", { name: "Add to team" }));

        // then
        expect(addition.mutate).toHaveBeenCalledWith(
            { teamId: "team-crew", user_id: "user-kit", role: "lead" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
        expect(onDone).toHaveBeenCalledOnce();
    });

    it("says who to choose when nobody is picked, without adding anyone", async () => {
        // given
        const user = userEvent.setup();
        const addition = makeMutationState();
        useAddTeamMember.mockReturnValue(addition);
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Add to team" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Choose who to add.");
        expect(screen.getByLabelText("Person")).toBeInvalid();
        expect(addition.mutate).not.toHaveBeenCalled();
    });

    it("offers a lead only the member role and only people they outrank", () => {
        // when
        renderDialog({ viewer: lead });

        // then
        expect(personOptions()).toEqual(["Choose a person", "Kit (kit@example.test)"]);
        expect(
            within(screen.getByLabelText("Team role"))
                .getAllByRole("option")
                .map(option => option.textContent),
        ).toEqual(["Member"]);
    });

    it("shows the server's refusal", () => {
        // given
        useAddTeamMember.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 409, message: "already on the team" }) }),
        );

        // when
        renderDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("already on the team");
    });

    it("closes from the cancel button", async () => {
        // given
        const user = userEvent.setup();
        const { onClose } = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });
});

describe("AddPeopleDialog inviting someone new", () => {
    it("switches to the invite form", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Invite by email" }));

        // then
        expect(screen.getByRole("button", { name: "Invite by email" })).toHaveAttribute("aria-pressed", "true");
        expect(screen.getByLabelText("Email address")).toBeInTheDocument();
        expect(screen.queryByLabelText("Person")).not.toBeInTheDocument();
    });

    it("goes straight to the invite form when everyone is already on the team", () => {
        // when
        renderDialog({ people: [sam, lee], members: [leeOnTeam] });

        // then
        expect(screen.getByText(/is already on this team, so invite someone new/)).toBeInTheDocument();
        expect(screen.getByLabelText("Email address")).toBeInTheDocument();
        expect(screen.queryByRole("group", { name: "Who to add" })).not.toBeInTheDocument();
    });

    it("invites the address into the organisation and this team, and reports it", async () => {
        // given
        const user = userEvent.setup();
        const invite = makeMutationState();
        invite.mutate.mockImplementation(succeed);
        useCreateInvite.mockReturnValue(invite);
        const { onDone } = renderDialog({ people: [sam] });

        // when
        await user.type(screen.getByLabelText("Email address"), " new@example.test ");
        await user.click(screen.getByRole("button", { name: "Send invite" }));

        // then
        expect(invite.mutate).toHaveBeenCalledWith(
            { orgId: "org-harbour", role: "member", team_id: "team-crew", email: "new@example.test" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
        expect(onDone).toHaveBeenCalledOnce();
    });

    it("asks for an address before sending", async () => {
        // given
        const user = userEvent.setup();
        const invite = makeMutationState();
        useCreateInvite.mockReturnValue(invite);
        renderDialog({ people: [sam] });

        // when
        await user.click(screen.getByRole("button", { name: "Send invite" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter an email address.");
        expect(invite.mutate).not.toHaveBeenCalled();
    });

    it("says the site cannot send email when the send is refused for that", () => {
        // given
        useCreateInvite.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 503, message: "email unavailable", code: "email_unavailable" }),
            }),
        );

        // when
        renderDialog({ people: [sam] });

        // then
        expect(screen.getAllByRole("alert")[0]).toHaveTextContent(
            "This site cannot send email yet, so nothing was sent.",
        );
        expect(screen.getByRole("button", { name: "Create a link instead" })).toBeInTheDocument();
    });

    it("creates a link invite for this team and lets the viewer copy it", async () => {
        // given
        const user = userEvent.setup();
        const invite = makeMutationState({
            isSuccess: true,
            data: makeCreatedInvite({ url: "https://retrotide.test/invite#abc" }),
        });
        useCreateInvite.mockReturnValue(invite);
        renderDialog({ people: [sam] });

        // when
        await user.click(screen.getByRole("button", { name: "Create a link instead" }));
        await user.click(screen.getByRole("button", { name: "Copy link" }));

        // then
        expect(invite.mutate).toHaveBeenCalledWith({ orgId: "org-harbour", role: "member", team_id: "team-crew" });
        expect(screen.getByLabelText("Invite link")).toHaveValue("https://retrotide.test/invite#abc");
        expect(copyText).toHaveBeenCalledWith("https://retrotide.test/invite#abc");
        expect(await screen.findByText("Link copied.")).toBeInTheDocument();
    });
});

describe("AddPeopleDialog for a lead who cannot invite", () => {
    it("only offers people already in the organisation", () => {
        // when
        renderDialog({ viewer: lead });

        // then
        expect(screen.queryByRole("button", { name: "Invite by email" })).not.toBeInTheDocument();
        expect(screen.getByLabelText("Person")).toBeInTheDocument();
    });

    it("says an owner or admin can invite someone new when there is nobody left to add", () => {
        // when
        renderDialog({ viewer: lead, people: [lee], members: [leeOnTeam] });

        // then
        expect(screen.getByText(/Ask an owner or admin to invite someone new/)).toBeInTheDocument();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
    });
});

describe("AddPeopleDialog loading", () => {
    it("says it is loading the organisation's members", () => {
        // given
        useOrganisationMembers.mockReturnValue(makeQueryState<OrganisationMemberListResponse>({ loading: true }));

        // when
        renderWithProviders(
            <AddPeopleDialog
                open
                organisation={harbour}
                team={crew}
                viewer={owner}
                members={[]}
                onDone={vi.fn()}
                onClose={vi.fn()}
            />,
        );

        // then
        expect(screen.getByText("Loading the organisation's members...")).toBeInTheDocument();
    });
});
