import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeAdminInvitation,
    makeAdminInvitationList,
    makeApiError,
    makeMutationState,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminInvitationListResponse, SiteMode } from "../../../types/api";
import { AdminInvitationsPage } from "./AdminInvitationsPage";

const { useAdminInvitations, useInviteUser, useResendInvitation, useCancelInvitation } = vi.hoisted(() => ({
    useAdminInvitations: vi.fn(),
    useInviteUser: vi.fn(),
    useResendInvitation: vi.fn(),
    useCancelInvitation: vi.fn(),
}));

vi.mock("../../../hooks/queries/admin", () => ({ useAdminInvitations }));
vi.mock("../../../hooks/mutations/admin", () => ({ useInviteUser, useResendInvitation, useCancelInvitation }));

const kit = makeAdminInvitation({ id: "invite-kit", email: "kit@example.test" });

function renderPage(siteMode: SiteMode = "private") {
    return renderWithProviders(
        <Routes>
            <Route path="/admin" element={<p>the admin home</p>} />
            <Route path="/admin/invitations" element={<AdminInvitationsPage />} />
        </Routes>,
        {
            user: makeSessionUser(),
            permissions: ["view_admin_panel", "manage_user_account"],
            route: "/admin/invitations",
            siteInfo: { site_mode: siteMode },
        },
    );
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

function showInvitations(listing: AdminInvitationListResponse) {
    useAdminInvitations.mockReturnValue(makeQueryState<AdminInvitationListResponse>({ data: listing }));
}

beforeEach(() => {
    vi.clearAllMocks();
    showInvitations(makeAdminInvitationList([kit]));
    useInviteUser.mockReturnValue(makeMutationState());
    useResendInvitation.mockReturnValue(makeMutationState());
    useCancelInvitation.mockReturnValue(makeMutationState());
});

describe("AdminInvitationsPage", () => {
    it("heads the page", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Invitations" })).toBeInTheDocument();
    });

    it("sends the viewer back to the admin home on a public site, where nobody needs an invitation", () => {
        // when
        renderPage("public");

        // then
        expect(screen.getByText("the admin home")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Invitations" })).not.toBeInTheDocument();
        expect(useAdminInvitations).not.toHaveBeenCalled();
    });

    it("asks for the first page of pending invitations", () => {
        // when
        renderPage();

        // then
        expect(useAdminInvitations).toHaveBeenLastCalledWith({ limit: 20, offset: 0 });
    });

    it("says it is loading while the first page is on its way", () => {
        // given
        useAdminInvitations.mockReturnValue(makeQueryState<AdminInvitationListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading invitations...")).toBeInTheDocument();
    });

    it("lists each pending invitation with its address, its state and when it expires", () => {
        // given
        const stale = makeAdminInvitation({
            id: "invite-robin",
            email: "robin@example.test",
            expired: true,
            expires_at: "2026-09-20T12:00:00Z",
        });
        showInvitations(makeAdminInvitationList([kit, stale]));

        // when
        renderPage();

        // then
        const table = screen.getByRole("table", { name: "Pending invitations" });
        const [, kitRow, robinRow] = within(table).getAllByRole("row");
        expect(kitRow).toHaveTextContent("kit@example.test");
        expect(kitRow).toHaveTextContent("Pending");
        expect(kitRow).toHaveTextContent("3 October 2026");
        expect(kitRow).not.toHaveTextContent("Expired");
        expect(robinRow).toHaveTextContent("robin@example.test");
        expect(robinRow).toHaveTextContent("Expired");
        expect(robinRow).toHaveTextContent("20 September 2026");
    });

    it("says so when nobody is waiting on an invitation", () => {
        // given
        showInvitations(makeAdminInvitationList([]));

        // when
        renderPage();

        // then
        expect(screen.getByText("No pending invitations.")).toBeInTheDocument();
        expect(screen.queryByRole("table", { name: "Pending invitations" })).not.toBeInTheDocument();
    });

    it("moves through the pages", async () => {
        // given
        const user = userEvent.setup();
        showInvitations(makeAdminInvitationList([kit], { total: 45 }));
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(useAdminInvitations).toHaveBeenLastCalledWith({ limit: 20, offset: 20 });
    });

    it("shows the server's reason when the invitations could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<AdminInvitationListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the invitations" }),
        });
        useAdminInvitations.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the invitations");
        expect(screen.queryByText("Loading invitations...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});

describe("AdminInvitationsPage inviting someone", () => {
    it("invites the address entered, without its spaces, and clears the field once it is sent", async () => {
        // given
        const invite = makeMutationState({ mutate: vi.fn(succeed) });
        useInviteUser.mockReturnValue(invite);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.type(screen.getByLabelText("Email address"), " robin@example.test ");
        await user.click(screen.getByRole("button", { name: "Send invitation" }));

        // then
        expect(invite.mutate).toHaveBeenCalledExactlyOnceWith({ email: "robin@example.test" }, expect.anything());
        expect(screen.getByLabelText("Email address")).toHaveValue("");
    });

    it("says what is missing, ties it to the field and moves focus to it, without sending anything", async () => {
        // given
        const invite = makeMutationState();
        useInviteUser.mockReturnValue(invite);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Send invitation" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter an email address.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByLabelText("Email address")).toBeInvalid();
        expect(invite.mutate).not.toHaveBeenCalled();
        expect(invite.reset).toHaveBeenCalled();
    });

    it("says the invitation was sent", () => {
        // given
        useInviteUser.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Invitation sent.");
    });

    it("holds the button while the invitation is sent", () => {
        // given
        useInviteUser.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("button", { name: "Send invitation" })).toBeDisabled();
    });

    it.each([
        { status: 409, message: "that email address already has an account", code: "account_exists" },
        { status: 409, message: "that email address already has a pending invitation", code: "invitation_pending" },
        { status: 400, message: "a valid email address is required" },
    ])("ties the server's refusal of the address to the field: $message", refusal => {
        // given
        useInviteUser.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
    });

    it.each([
        { status: 409, message: "invitations are only for a private site", code: "site_public" },
        { status: 503, message: "email is not available on this site" },
        { status: 500, message: "the invitation was saved, but its email was not sent, so resend it" },
    ])("shows any other refusal without blaming the address: $message", refusal => {
        // given
        useInviteUser.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Email address")).not.toBeInvalid();
    });

    it("says plainly that nothing was sent when the site cannot send email", () => {
        // given
        const error = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });
        useInviteUser.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("This site cannot send email yet, so nothing was sent.");
        expect(screen.getByLabelText("Email address")).not.toBeInvalid();
    });
});

describe("AdminInvitationsPage resending an invitation", () => {
    it("sends the invitation again", async () => {
        // given
        const resend = makeMutationState();
        useResendInvitation.mockReturnValue(resend);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Resend invitation to kit@example.test" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith({ invitationId: "invite-kit" });
    });

    it("says the invitation was sent again", () => {
        // given
        useResendInvitation.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Invitation sent again.");
    });

    it("holds the resend buttons while one is being sent", () => {
        // given
        useResendInvitation.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("button", { name: "Resend invitation to kit@example.test" })).toBeDisabled();
    });

    it("shows the server's reason when the invitation could not be sent again", () => {
        // given
        const error = makeApiError({ status: 404, message: "invitation not found" });
        useResendInvitation.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("invitation not found");
    });

    it("says plainly that nothing was sent again when the site cannot send email", () => {
        // given
        const error = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });
        useResendInvitation.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("This site cannot send email yet, so nothing was sent.");
    });
});

describe("AdminInvitationsPage cancelling an invitation", () => {
    it("asks before cancelling the invitation, then cancels it", async () => {
        // given
        const cancel = makeMutationState({ mutate: vi.fn(succeed) });
        useCancelInvitation.mockReturnValue(cancel);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel invitation to kit@example.test" }));
        const dialog = screen.getByRole("dialog", { name: "Cancel invitation" });
        const body = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Cancel invitation" }));

        // then
        expect(body).toContain("The invitation to kit@example.test stops working.");
        expect(cancel.mutate).toHaveBeenCalledWith({ invitationId: "invite-kit" }, expect.anything());
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the invitation when the viewer changes their mind", async () => {
        // given
        const cancel = makeMutationState();
        useCancelInvitation.mockReturnValue(cancel);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel invitation to kit@example.test" }));
        await user.click(screen.getByRole("button", { name: "Keep invitation" }));

        // then
        expect(cancel.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when it fails", async () => {
        // given
        const error = makeApiError({ status: 500, message: "failed to cancel the invitation" });
        useCancelInvitation.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel invitation to kit@example.test" }));

        // then
        expect(screen.getByRole("dialog", { name: "Cancel invitation" })).toHaveTextContent(
            "failed to cancel the invitation",
        );
    });

    it("says the invitation was cancelled", () => {
        // given
        useCancelInvitation.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Invitation cancelled.");
    });
});
