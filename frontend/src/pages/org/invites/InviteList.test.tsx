import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeOrganisationInvite,
    makeQueryState,
    makeTeam,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { OrganisationInviteListResponse, OrganisationInviteResponse, OrgRole } from "../../../types/api";
import { InviteList } from "./InviteList";

const { useOrganisationInvites, useResendInvite, useRevokeInvite } = vi.hoisted(() => ({
    useOrganisationInvites: vi.fn(),
    useResendInvite: vi.fn(),
    useRevokeInvite: vi.fn(),
}));

vi.mock("../../../hooks/queries/invites", () => ({ useOrganisationInvites }));
vi.mock("../../../hooks/mutations/invites", () => ({ useResendInvite, useRevokeInvite }));

const harbour = makeOrganisation({ id: "org-harbour" });
const crew = makeTeam({ id: "team-crew", name: "Crew" });

const kitInvite = makeOrganisationInvite({
    id: "invite-kit",
    email: "kit@example.test",
    role: "admin",
    team_id: "team-crew",
    max_uses: 1,
    use_count: 0,
    expires_at: "2026-10-03T12:00:00Z",
});
const linkInvite = makeOrganisationInvite({
    id: "invite-link",
    email: undefined,
    role: "member",
    max_uses: undefined,
    use_count: 3,
    expired: true,
    expires_at: "2026-09-20T12:00:00Z",
});

function showInvites(invites: OrganisationInviteResponse[]) {
    useOrganisationInvites.mockReturnValue(makeQueryState<OrganisationInviteListResponse>({ data: { invites } }));
}

function renderList(role: OrgRole = "owner") {
    return renderWithProviders(<InviteList organisation={harbour} role={role} teams={[crew]} />);
}

function rowOf(text: string): HTMLElement {
    const row = within(screen.getByRole("table", { name: "Pending invites" }))
        .getAllByRole("row")
        .find(candidate => within(candidate).queryByText(text) !== null);

    if (row === undefined) {
        throw new Error(`no row for ${text}`);
    }

    return row;
}

async function menuOf(user: ReturnType<typeof userEvent.setup>, text: string): Promise<string[]> {
    await user.click(within(rowOf(text)).getByRole("button", { name: /^Actions for / }));

    return within(screen.getByRole("menu"))
        .getAllByRole("menuitem")
        .map(item => item.textContent);
}

async function choose(user: ReturnType<typeof userEvent.setup>, text: string, action: string) {
    await user.click(within(rowOf(text)).getByRole("button", { name: /^Actions for / }));
    await user.click(screen.getByRole("menuitem", { name: action }));
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    vi.clearAllMocks();
    showInvites([kitInvite, linkInvite]);
    useResendInvite.mockReturnValue(makeMutationState());
    useRevokeInvite.mockReturnValue(makeMutationState());
});

describe("InviteList", () => {
    it("asks for the organisation's pending invites", () => {
        // when
        renderList();

        // then
        expect(useOrganisationInvites).toHaveBeenCalledWith("org-harbour");
        expect(screen.getByRole("heading", { level: 2, name: "Pending invites" })).toBeInTheDocument();
    });

    it("lists an email invite with its role, team, uses and expiry", () => {
        // when
        renderList();

        // then
        const row = rowOf("kit@example.test");
        expect(row).toHaveTextContent("Admin");
        expect(row).toHaveTextContent("Crew");
        expect(row).toHaveTextContent("0 of 1");
        expect(row).toHaveTextContent("3 October 2026");
        expect(row).not.toHaveTextContent("Expired");
    });

    it("lists a link invite with the times it was used and marks it expired", () => {
        // when
        renderList();

        // then
        const row = rowOf("Invite link");
        expect(row).toHaveTextContent("Member");
        expect(row).toHaveTextContent("No team");
        expect(row).toHaveTextContent("3 (no limit)");
        expect(row).toHaveTextContent("Expired");
        expect(row).toHaveTextContent("20 September 2026");
    });

    it("says the team is unknown when it is not among the organisation's teams", () => {
        // given
        showInvites([makeOrganisationInvite({ email: "kit@example.test", team_id: "team-gone" })]);

        // when
        renderList();

        // then
        expect(rowOf("kit@example.test")).toHaveTextContent("Unknown team");
    });

    it("says it is loading while the invites are on their way", () => {
        // given
        useOrganisationInvites.mockReturnValue(makeQueryState<OrganisationInviteListResponse>({ loading: true }));

        // when
        renderList();

        // then
        expect(screen.getByText("Loading invites...")).toBeInTheDocument();
    });

    it("says so when no invite is pending", () => {
        // given
        showInvites([]);

        // when
        renderList();

        // then
        expect(screen.getByText("No pending invites.")).toBeInTheDocument();
        expect(screen.queryByRole("table", { name: "Pending invites" })).not.toBeInTheDocument();
    });

    it("shows the server's reason when the invites could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<OrganisationInviteListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the invites" }),
        });
        useOrganisationInvites.mockReturnValue(failed);
        renderList();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the invites");
        expect(screen.queryByText("Loading invites...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});

describe("InviteList resending an invite", () => {
    it("offers to resend only the invites sent to an address, each in its own menu", async () => {
        // given
        const user = userEvent.setup();
        renderList();

        // when
        const kitActions = await menuOf(user, "kit@example.test");
        await user.keyboard("{Escape}");
        const linkActions = await menuOf(user, "Invite link");

        // then
        expect(kitActions).toEqual(["Resend", "Revoke"]);
        expect(linkActions).toEqual(["Revoke"]);
        expect(screen.getByRole("button", { name: "Actions for kit@example.test" })).toBeInTheDocument();
        expect(
            screen.getByRole("button", { name: /^Actions for the invite link that expires 20 September 2026/ }),
        ).toBeInTheDocument();
    });

    it("offers an admin neither resend nor revoke of an admin invite, which only the owner may manage", async () => {
        // given
        const memberInvite = makeOrganisationInvite({
            id: "invite-robin",
            email: "robin@example.test",
            role: "member",
        });
        showInvites([kitInvite, memberInvite]);
        const user = userEvent.setup();
        renderList("admin");

        // when
        const robinActions = await menuOf(user, "robin@example.test");

        // then
        expect(within(rowOf("kit@example.test")).queryByRole("button")).not.toBeInTheDocument();
        expect(robinActions).toEqual(["Resend", "Revoke"]);
    });

    it("drops the actions column when the viewer may manage none of the invites", () => {
        // given
        showInvites([kitInvite]);

        // when
        renderList("admin");

        // then
        expect(screen.queryByRole("columnheader", { name: "Actions" })).not.toBeInTheDocument();
        expect(within(rowOf("kit@example.test")).queryByRole("button")).not.toBeInTheDocument();
    });

    it("sends the invite again and says so", async () => {
        // given
        const resend = makeMutationState({
            mutate: vi.fn(() => {
                useResendInvite.mockReturnValue(makeMutationState({ isSuccess: true }));
            }),
        });
        useResendInvite.mockReturnValue(resend);
        const user = userEvent.setup();
        const { rerender } = renderList();

        // when
        await choose(user, "kit@example.test", "Resend");
        rerender(<InviteList organisation={harbour} role="owner" teams={[crew]} />);

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith({ orgId: "org-harbour", inviteId: "invite-kit" });
        expect(screen.getByRole("status")).toHaveTextContent("Invite sent again to kit@example.test.");
    });

    it("offers no resend while one is being sent", async () => {
        // given
        useResendInvite.mockReturnValue(makeMutationState({ isPending: true }));
        const user = userEvent.setup();
        renderList();

        // when
        const kitActions = await menuOf(user, "kit@example.test");

        // then
        expect(kitActions).toEqual(["Revoke"]);
    });

    it("shows the server's reason when the invite could not be sent again", () => {
        // given
        const error = makeApiError({ status: 500, message: "the invite was saved, but its email was not sent" });
        useResendInvite.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderList();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the invite was saved, but its email was not sent");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});

describe("InviteList revoking an invite", () => {
    it("asks before revoking an email invite, revokes it and says so", async () => {
        // given
        const revoke = makeMutationState({
            mutate: vi.fn((variables: unknown, options?: { onSuccess?: () => void }) => {
                useRevokeInvite.mockReturnValue(makeMutationState({ isSuccess: true }));
                succeed(variables, options);
            }),
        });
        useRevokeInvite.mockReturnValue(revoke);
        const user = userEvent.setup();
        renderList();

        // when
        await choose(user, "kit@example.test", "Revoke");
        const dialog = screen.getByRole("dialog", { name: "Revoke this invite?" });
        const body = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Revoke" }));

        // then
        expect(body).toContain("The invite to kit@example.test stops working.");
        expect(revoke.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", inviteId: "invite-kit" },
            expect.anything(),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Invite revoked.");
    });

    it("says a revoked link stops working for everyone who has it", async () => {
        // given
        const user = userEvent.setup();
        renderList();

        // when
        await choose(user, "Invite link", "Revoke");

        // then
        expect(screen.getByRole("dialog", { name: "Revoke this invite?" })).toHaveTextContent(
            "The invite link stops working, so nobody else can join with it.",
        );
    });

    it("keeps the invite when the viewer changes their mind", async () => {
        // given
        const revoke = makeMutationState();
        useRevokeInvite.mockReturnValue(revoke);
        const user = userEvent.setup();
        renderList();

        // when
        await choose(user, "kit@example.test", "Revoke");
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(revoke.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when revoking fails", async () => {
        // given
        const error = makeApiError({ status: 404, message: "invite not found" });
        useRevokeInvite.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderList();

        // when
        await choose(user, "kit@example.test", "Revoke");

        // then
        expect(screen.getByRole("dialog", { name: "Revoke this invite?" })).toHaveTextContent("invite not found");
    });
});
