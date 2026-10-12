import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeOrganisationMember,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type {
    OrganisationMemberListResponse,
    OrganisationMemberResponse,
    OrgRole,
    StaffRole,
} from "../../../types/api";
import { OrgMembersPage } from "./OrgMembersPage";

const { useOrganisationMembers, useChangeMemberRole, useRemoveMember, useTransferOwnership, useLeaveOrganisation } =
    vi.hoisted(() => ({
        useOrganisationMembers: vi.fn(),
        useChangeMemberRole: vi.fn(),
        useRemoveMember: vi.fn(),
        useTransferOwnership: vi.fn(),
        useLeaveOrganisation: vi.fn(),
    }));

vi.mock("../../../hooks/queries/organisations", () => ({ useOrganisationMembers }));
vi.mock("../../../hooks/mutations/organisations", () => ({
    useChangeMemberRole,
    useRemoveMember,
    useTransferOwnership,
    useLeaveOrganisation,
}));

const viewer = makeSessionUser({ id: "user-sam", display_name: "Sam" });
const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });
const kit = makeOrganisationMember({
    user_id: "user-kit",
    display_name: "Kit",
    email: "kit@example.test",
    role: "owner",
});
const alex = makeOrganisationMember({
    user_id: "user-alex",
    display_name: "Alex",
    email: "alex@example.test",
    role: "admin",
});
const robin = makeOrganisationMember({
    user_id: "user-robin",
    display_name: "Robin",
    email: "robin@example.test",
    role: "member",
    joined_at: "2026-09-02T10:00:00Z",
});

function membersSeenBy(role: OrgRole): OrganisationMemberResponse[] {
    const sam = makeOrganisationMember({
        user_id: viewer.id,
        display_name: "Sam",
        email: "sam@example.test",
        role,
    });

    if (role === "owner") {
        return [sam, alex, robin];
    }

    return [kit, sam, alex, robin];
}

function showMembers(members: OrganisationMemberResponse[]) {
    useOrganisationMembers.mockReturnValue(makeQueryState<OrganisationMemberListResponse>({ data: { members } }));
}

function renderPage(role: OrgRole = "owner", deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, role, deleted_at: deletedAt });
    showMembers(membersSeenBy(role));

    return renderWithProviders(<OrgMembersPage />, { user: viewer, org: { organisations: [current], current } });
}

function renderMembers(members: OrganisationMemberResponse[], siteRole: StaffRole | null = null) {
    const current = makeOrganisation({ ...harbour, role: "owner" });
    showMembers(members);

    return renderWithProviders(<OrgMembersPage />, {
        user: viewer,
        auth: { siteRole },
        org: { organisations: [current], current },
    });
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

function rowOf(name: string): HTMLElement {
    const table = screen.getByRole("table", { name: "Members" });
    const row = within(table)
        .getAllByRole("row")
        .find(candidate => within(candidate).queryByText(name) !== null);

    if (row === undefined) {
        throw new Error(`no row for ${name}`);
    }

    return row;
}

async function menuOf(user: ReturnType<typeof userEvent.setup>, name: string): Promise<string[]> {
    await user.click(within(rowOf(name)).getByRole("button", { name: `Actions for ${name}` }));

    return within(screen.getByRole("menu", { name: `Actions for ${name}` }))
        .getAllByRole("menuitem")
        .map(item => item.textContent);
}

async function choose(user: ReturnType<typeof userEvent.setup>, name: string, action: string) {
    await user.click(within(rowOf(name)).getByRole("button", { name: `Actions for ${name}` }));
    await user.click(screen.getByRole("menuitem", { name: action }));
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    useChangeMemberRole.mockReturnValue(makeMutationState());
    useRemoveMember.mockReturnValue(makeMutationState());
    useTransferOwnership.mockReturnValue(makeMutationState());
    useLeaveOrganisation.mockReturnValue(makeMutationState());
});

describe("OrgMembersPage", () => {
    it("heads the page and asks for the current organisation's members", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Members" })).toBeInTheDocument();
        expect(document.title).toContain("Members");
        expect(useOrganisationMembers).toHaveBeenCalledWith("org-harbour");
    });

    it("lists each member with their address, role and when they joined, and marks the viewer", () => {
        // when
        renderPage();

        // then
        const robinRow = rowOf("Robin");
        expect(robinRow).toHaveTextContent("robin@example.test");
        expect(robinRow).toHaveTextContent("Member");
        expect(robinRow).toHaveTextContent("02/09/2026");
        expect(rowOf("Alex")).toHaveTextContent("Admin");
        expect(rowOf("Sam")).toHaveTextContent("Sam (you)");
        expect(rowOf("Sam")).toHaveTextContent("Owner");
    });

    it("says it is loading while the members are on their way", () => {
        // given
        useOrganisationMembers.mockReturnValue(makeQueryState<OrganisationMemberListResponse>({ loading: true }));

        // when
        renderWithProviders(<OrgMembersPage />, { user: viewer, org: { organisations: [harbour] } });

        // then
        expect(screen.getByText("Loading members...")).toBeInTheDocument();
    });

    it("shows the server's reason when the members could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<OrganisationMemberListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the members" }),
        });
        useOrganisationMembers.mockReturnValue(failed);
        renderWithProviders(<OrgMembersPage />, { user: viewer, org: { organisations: [harbour] } });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the members");
        expect(screen.queryByText("Loading members...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("says the organisation is deleted instead of asking for its members", () => {
        // when
        renderPage("owner", "2026-09-20T12:00:00Z");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useOrganisationMembers).not.toHaveBeenCalled();
    });
});

describe("OrgMembersPage actions each viewer may take", () => {
    it("offers the owner every action on everyone but themselves, in a menu per member", async () => {
        // given
        const user = userEvent.setup();
        renderPage("owner");

        // when
        const robinActions = await menuOf(user, "Robin");
        await user.keyboard("{Escape}");
        const alexActions = await menuOf(user, "Alex");

        // then
        expect(robinActions).toEqual(["Make admin", "Make owner", "Remove"]);
        expect(alexActions).toEqual(["Make member", "Make owner", "Remove"]);
        expect(within(rowOf("Sam")).queryByRole("button")).not.toBeInTheDocument();
    });

    it("lets an admin remove a member, and nothing else, because only the owner makes or unmakes admins", async () => {
        // given
        const user = userEvent.setup();
        renderPage("admin");

        // when
        const robinActions = await menuOf(user, "Robin");
        await user.keyboard("{Escape}");
        const ownActions = await menuOf(user, "Sam");

        // then
        expect(robinActions).toEqual(["Remove"]);
        expect(ownActions).toEqual(["Leave organisation"]);
        expect(within(rowOf("Alex")).queryByRole("button")).not.toBeInTheDocument();
        expect(within(rowOf("Kit")).queryByRole("button")).not.toBeInTheDocument();
    });

    it("tells a plain member only owners and admins see who is in the organisation, and never asks for the list", () => {
        // when
        renderPage("member");

        // then
        expect(screen.getByText("Only the owner and admins can see who is in the organisation.")).toBeInTheDocument();
        expect(screen.queryByRole("table")).not.toBeInTheDocument();
        expect(useOrganisationMembers).not.toHaveBeenCalled();
    });

    it("never offers the owner the way to leave, since they must hand the organisation on first", () => {
        // when
        renderPage("owner");

        // then
        expect(screen.queryByRole("button", { name: "Actions for Sam" })).not.toBeInTheDocument();
    });

    it("hides every action on a member who is site staff, who stands above every organisation role", async () => {
        // given
        const sam = makeOrganisationMember({ user_id: viewer.id, display_name: "Sam", role: "owner" });
        const staffRobin = makeOrganisationMember({ ...robin, site_role: "support" });
        const user = userEvent.setup();
        renderMembers([sam, alex, staffRobin]);

        // when
        const alexActions = await menuOf(user, "Alex");

        // then
        expect(within(rowOf("Robin")).queryByRole("button")).not.toBeInTheDocument();
        expect(alexActions).toContain("Remove");
    });

    it("lets a viewer who is site staff act on staff below them, but not on staff at their own level", async () => {
        // given
        const sam = makeOrganisationMember({ user_id: viewer.id, display_name: "Sam", role: "owner" });
        const supportAlex = makeOrganisationMember({ ...alex, site_role: "support" });
        const staffRobin = makeOrganisationMember({ ...robin, site_role: "admin" });
        const user = userEvent.setup();
        renderMembers([sam, supportAlex, staffRobin], "admin");

        // when
        const alexActions = await menuOf(user, "Alex");

        // then
        expect(alexActions).toContain("Remove");
        expect(within(rowOf("Robin")).queryByRole("button")).not.toBeInTheDocument();
    });
});

describe("OrgMembersPage changing a role", () => {
    it("changes the member's role", async () => {
        // given
        const changeRole = makeMutationState();
        useChangeMemberRole.mockReturnValue(changeRole);
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Make admin");

        // then
        expect(changeRole.mutate).toHaveBeenCalledExactlyOnceWith({
            orgId: "org-harbour",
            userId: "user-robin",
            role: "admin",
        });
    });

    it("says what the member's role now is", () => {
        // given
        useChangeMemberRole.mockReturnValue(
            makeMutationState({
                isSuccess: true,
                variables: { orgId: "org-harbour", userId: "user-robin", role: "admin" },
            }),
        );

        // when
        renderPage("owner");

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Robin is now an admin.");
    });

    it("offers no role change while one is saved", async () => {
        // given
        useChangeMemberRole.mockReturnValue(makeMutationState({ isPending: true }));
        const user = userEvent.setup();
        renderPage("owner");

        // when
        const robinActions = await menuOf(user, "Robin");

        // then
        expect(robinActions).toEqual(["Make owner", "Remove"]);
    });

    it("shows the server's refusal as it comes and moves focus to it", () => {
        // given
        const error = makeApiError({ status: 403, message: "only the owner can make, unmake or remove an admin" });
        useChangeMemberRole.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderPage("owner");

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("only the owner can make, unmake or remove an admin");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});

describe("OrgMembersPage removing a member", () => {
    it("asks before removing the member, removes them and says so", async () => {
        // given
        const removal = makeMutationState({
            mutate: vi.fn((variables: unknown, options?: { onSuccess?: () => void }) => {
                useRemoveMember.mockReturnValue(makeMutationState({ isSuccess: true }));
                succeed(variables, options);
            }),
        });
        useRemoveMember.mockReturnValue(removal);
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Remove");
        const dialog = screen.getByRole("dialog", { name: "Remove Robin?" });
        const body = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Remove" }));

        // then
        expect(body).toContain("Robin loses access to Harbour and its teams.");
        expect(removal.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", userId: "user-robin" },
            expect.anything(),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Robin removed.");
    });

    it("keeps the member when the viewer changes their mind", async () => {
        // given
        const removal = makeMutationState();
        useRemoveMember.mockReturnValue(removal);
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Remove");
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(removal.mutate).not.toHaveBeenCalled();
        expect(removal.reset).toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when the removal fails", async () => {
        // given
        const error = makeApiError({ status: 404, message: "that person is not a member of the organisation" });
        useRemoveMember.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Remove");

        // then
        expect(screen.getByRole("dialog", { name: "Remove Robin?" })).toHaveTextContent(
            "that person is not a member of the organisation",
        );
    });
});

describe("OrgMembersPage making someone else the owner", () => {
    it("asks before making the member the owner, then does it and says so", async () => {
        // given
        const transfer = makeMutationState({
            mutate: vi.fn((variables: unknown, options?: { onSuccess?: () => void }) => {
                useTransferOwnership.mockReturnValue(
                    makeMutationState({ isSuccess: true, variables: { orgId: "org-harbour", user_id: "user-robin" } }),
                );
                succeed(variables, options);
            }),
        });
        useTransferOwnership.mockReturnValue(transfer);
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Make owner");
        const dialog = screen.getByRole("dialog", { name: "Make Robin the owner?" });
        const body = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Make owner" }));

        // then
        expect(body).toContain("Robin becomes the owner of Harbour. You become an admin");
        expect(transfer.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", user_id: "user-robin" },
            expect.anything(),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Robin is now the owner.");
    });

    it("keeps ownership when the viewer changes their mind", async () => {
        // given
        const transfer = makeMutationState();
        useTransferOwnership.mockReturnValue(transfer);
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Make owner");
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(transfer.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when the transfer fails", async () => {
        // given
        const error = makeApiError({
            status: 409,
            message:
                "ownership was not transferred, because the organisation, its owner or the new owner changed meanwhile",
        });
        useTransferOwnership.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Make owner");

        // then
        expect(screen.getByRole("dialog", { name: "Make Robin the owner?" })).toHaveTextContent(
            "ownership was not transferred",
        );
    });

    it("says plainly when the new owner already owns as many organisations as they can", async () => {
        // given
        const error = makeApiError({
            status: 409,
            message: "the new owner already owns as many organisations as they can",
            code: "new_owner_limit_reached",
        });
        useTransferOwnership.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderPage("owner");

        // when
        await choose(user, "Robin", "Make owner");

        // then
        expect(
            within(screen.getByRole("dialog", { name: "Make Robin the owner?" })).getByRole("alert"),
        ).toHaveTextContent("Robin already owns as many organisations as they can, so you stay the owner.");
    });
});

describe("OrgMembersPage leaving the organisation", () => {
    it("asks before leaving, then leaves", async () => {
        // given
        const leave = makeMutationState();
        useLeaveOrganisation.mockReturnValue(leave);
        const user = userEvent.setup();
        renderPage("admin");

        // when
        await choose(user, "Sam", "Leave organisation");
        const dialog = screen.getByRole("dialog", { name: "Leave Harbour?" });
        const body = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Leave organisation" }));

        // then
        expect(body).toContain("You lose access to Harbour and its teams. To come back, you need a new invite.");
        expect(leave.mutate).toHaveBeenCalledExactlyOnceWith({ orgId: "org-harbour" }, expect.anything());
    });

    it("stays when the viewer changes their mind", async () => {
        // given
        const leave = makeMutationState();
        useLeaveOrganisation.mockReturnValue(leave);
        const user = userEvent.setup();
        renderPage("admin");

        // when
        await choose(user, "Sam", "Leave organisation");
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(leave.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when leaving fails", async () => {
        // given
        const error = makeApiError({
            status: 409,
            message: "the owner cannot leave the organisation, so transfer ownership or delete it first",
        });
        useLeaveOrganisation.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderPage("admin");

        // when
        await choose(user, "Sam", "Leave organisation");

        // then
        expect(screen.getByRole("dialog", { name: "Leave Harbour?" })).toHaveTextContent(
            "the owner cannot leave the organisation",
        );
    });

    it("says the viewer left while the page is still up", () => {
        // given
        useLeaveOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderPage("admin");

        // then
        expect(screen.getByRole("status")).toHaveTextContent("You left Harbour.");
    });
});
