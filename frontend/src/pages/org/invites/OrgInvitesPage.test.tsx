import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeOrganisationInvite,
    makeQueryState,
    makeSessionUser,
    makeTeam,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { OrganisationInviteListResponse, OrgRole, TeamListResponse } from "../../../types/api";
import { OrgInvitesPage } from "./OrgInvitesPage";

const { useTeams, useOrganisationInvites, useCreateInvite, useResendInvite, useRevokeInvite } = vi.hoisted(() => ({
    useTeams: vi.fn(),
    useOrganisationInvites: vi.fn(),
    useCreateInvite: vi.fn(),
    useResendInvite: vi.fn(),
    useRevokeInvite: vi.fn(),
}));

vi.mock("../../../hooks/queries/teams", () => ({ useTeams }));
vi.mock("../../../hooks/queries/invites", () => ({ useOrganisationInvites }));
vi.mock("../../../hooks/mutations/invites", () => ({ useCreateInvite, useResendInvite, useRevokeInvite }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });
const crew = makeTeam({ id: "team-crew", name: "Crew" });
const galley = makeTeam({ id: "team-galley", name: "Galley", archived_at: "2026-09-10T09:00:00Z" });

function renderPage(role: OrgRole = "owner", deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, role, deleted_at: deletedAt });

    return renderWithProviders(<OrgInvitesPage />, {
        user: makeSessionUser(),
        org: { organisations: [current], current },
    });
}

type InviteDialog = "Invite by email" | "Invite with a link";

const OPENERS: Readonly<Record<InviteDialog, string>> = {
    "Invite by email": "Invite by email",
    "Invite with a link": "Create invite link",
};

async function form(name: InviteDialog): Promise<HTMLElement> {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: OPENERS[name] }));

    return screen.getByRole("dialog", { name });
}

async function closeDialog() {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
}

function optionNames(scope: HTMLElement, label: string): string[] {
    return within(within(scope).getByLabelText(label))
        .getAllByRole("option")
        .map(option => option.textContent ?? "");
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ data: { teams: [crew, galley] } }));
    useOrganisationInvites.mockReturnValue(
        makeQueryState<OrganisationInviteListResponse>({
            data: { invites: [makeOrganisationInvite({ email: "kit@example.test", team_id: "team-galley" })] },
        }),
    );
    useCreateInvite.mockReturnValue(makeMutationState());
    useResendInvite.mockReturnValue(makeMutationState());
    useRevokeInvite.mockReturnValue(makeMutationState());
});

describe("OrgInvitesPage", () => {
    it("heads the page and offers both ways to invite as buttons, with the pending invites below", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Invites" })).toBeInTheDocument();
        expect(document.title).toContain("Invites");
        expect(screen.getByRole("button", { name: "Invite by email" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Create invite link" })).toBeInTheDocument();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("table", { name: "Pending invites" })).toBeInTheDocument();
        expect(useTeams).toHaveBeenCalledWith("org-harbour");
        expect(useOrganisationInvites).toHaveBeenCalledWith("org-harbour");
    });

    it("turns both ways to invite off, and says why, while the site cannot send email", () => {
        // given
        const current = makeOrganisation({ ...harbour, role: "owner" });

        // when
        renderWithProviders(<OrgInvitesPage />, {
            user: makeSessionUser(),
            org: { organisations: [current], current },
            siteInfo: { email_enabled: false },
        });

        // then
        expect(screen.getByRole("button", { name: "Invite by email" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Create invite link" })).toBeDisabled();
        expect(screen.getByText(/Invites are off until this site can send email/)).toBeInTheDocument();
        expect(screen.getByRole("table", { name: "Pending invites" })).toBeInTheDocument();
    });

    it("opens each way to invite in a dialog of its own and closes it again", async () => {
        // given
        renderPage();

        // when
        const email = await form("Invite by email");
        await closeDialog();

        // then
        expect(email).not.toBeInTheDocument();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(await form("Invite with a link")).toBeInTheDocument();
    });

    it("lets the owner invite admins and members", async () => {
        // given
        renderPage("owner");

        // when
        const emailRoles = optionNames(await form("Invite by email"), "Role");
        await closeDialog();
        const linkRoles = optionNames(await form("Invite with a link"), "Role");

        // then
        expect(emailRoles).toEqual(["Admin", "Member"]);
        expect(linkRoles).toEqual(["Admin", "Member"]);
    });

    it("lets an admin invite members only", async () => {
        // given
        renderPage("admin");

        // when
        const emailRoles = optionNames(await form("Invite by email"), "Role");
        await closeDialog();
        const linkRoles = optionNames(await form("Invite with a link"), "Role");

        // then
        expect(emailRoles).toEqual(["Member"]);
        expect(linkRoles).toEqual(["Member"]);
    });

    it("offers only active teams to invite into, and still names an archived team in the list", async () => {
        // given
        renderPage();

        // when
        const emailTeams = optionNames(await form("Invite by email"), "Team");
        await closeDialog();
        const linkTeams = optionNames(await form("Invite with a link"), "Team");

        // then
        expect(emailTeams).toEqual(["No team", "Crew"]);
        expect(linkTeams).toEqual(["No team", "Crew"]);
        expect(screen.getByRole("table", { name: "Pending invites" })).toHaveTextContent("Galley");
    });

    it("tells a member only the owner and admins invite people, and asks for no invites", () => {
        // when
        renderPage("member");

        // then
        expect(screen.getByText("Only the owner and admins can invite people.")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Invite by email" })).not.toBeInTheDocument();
        expect(useOrganisationInvites).not.toHaveBeenCalled();
    });

    it("says the organisation is deleted instead of asking for its invites", () => {
        // when
        renderPage("owner", "2026-09-20T12:00:00Z");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useOrganisationInvites).not.toHaveBeenCalled();
        expect(useTeams).not.toHaveBeenCalled();
    });

    it("shows the server's reason when the teams could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<TeamListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the teams" }),
        });
        useTeams.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the teams");
        expect(failed.refetch).toHaveBeenCalledOnce();
        expect(optionNames(await form("Invite by email"), "Team")).toEqual(["No team"]);
    });
});
