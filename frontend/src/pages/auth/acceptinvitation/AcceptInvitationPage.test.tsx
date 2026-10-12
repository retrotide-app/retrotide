import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeQueryState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { InvitationResponse } from "../../../types/api";
import { AcceptInvitationPage } from "./AcceptInvitationPage";

const { useInvitation, useAcceptInvitation, readLocationToken, clearLocationToken } = vi.hoisted(() => ({
    useInvitation: vi.fn(),
    useAcceptInvitation: vi.fn(),
    readLocationToken: vi.fn(),
    clearLocationToken: vi.fn(),
}));

vi.mock("../../../hooks/queries/auth", () => ({ useInvitation }));
vi.mock("../../../hooks/mutations/auth", () => ({ useAcceptInvitation }));
vi.mock("../../../platform/location", () => ({ readLocationToken, clearLocationToken }));

const invitation: InvitationResponse = { email: "kit@example.test", expires_at: "2026-10-03T12:00:00Z" };

async function fillIn(user: ReturnType<typeof userEvent.setup>) {
    await user.type(screen.getByLabelText("Name"), " Kit ");
    await user.type(screen.getByLabelText("Password"), "harbour-lights");
}

beforeEach(() => {
    useInvitation.mockReturnValue(makeQueryState<InvitationResponse>({ data: invitation }));
    useAcceptInvitation.mockReturnValue(makeMutationState());
    readLocationToken.mockReset();
    readLocationToken.mockReturnValue("invite-token-1");
    clearLocationToken.mockReset();
});

describe("AcceptInvitationPage", () => {
    it("looks the invitation up with the code from the link", () => {
        // given
        const route = "/accept-invitation";

        // when
        renderWithProviders(<AcceptInvitationPage />, { route });

        // then
        expect(useInvitation).toHaveBeenCalledWith("invite-token-1");
    });

    it("takes the code out of the address bar once it has read it", () => {
        // given
        const route = "/accept-invitation";

        // when
        renderWithProviders(<AcceptInvitationPage />, { route });

        // then
        expect(clearLocationToken).toHaveBeenCalled();
    });

    it("says it is checking the invitation while the server answers", () => {
        // given
        useInvitation.mockReturnValue(makeQueryState<InvitationResponse>({ loading: true }));

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Create your account" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Checking your invitation...");
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
    });

    it("shows the invited address, which cannot be changed, and asks for a name and a password", () => {
        // given
        const route = "/accept-invitation";

        // when
        renderWithProviders(<AcceptInvitationPage />, { route });

        // then
        const email = screen.getByLabelText("Email address");
        expect(email).toHaveValue("kit@example.test");
        expect(email).toHaveAttribute("readonly");
        expect(email).toHaveAccessibleDescription("Your invitation was sent to this address.");
        expect(screen.getByLabelText("Name")).toHaveAttribute("maxlength", "40");
        expect(screen.getByLabelText("Password")).toHaveAttribute("type", "password");
        expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
    });

    it("creates the account with the code, the name without its spaces and the password", async () => {
        // given
        const accept = makeMutationState();
        useAcceptInvitation.mockReturnValue(accept);
        const user = userEvent.setup();
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(accept.mutate).toHaveBeenCalledExactlyOnceWith({
            token: "invite-token-1",
            display_name: "Kit",
            password: "harbour-lights",
        });
    });

    it("keeps the invited address when someone types into it", async () => {
        // given
        const accept = makeMutationState();
        useAcceptInvitation.mockReturnValue(accept);
        const user = userEvent.setup();
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // when
        await user.type(screen.getByLabelText("Email address"), "robin");

        // then
        expect(screen.getByLabelText("Email address")).toHaveValue("kit@example.test");
    });

    it("says what is missing, ties each message to its field and moves focus to the summary", async () => {
        // given
        const accept = makeMutationState();
        useAcceptInvitation.mockReturnValue(accept);
        const user = userEvent.setup();
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // when
        await user.type(screen.getByLabelText("Name"), "   ");
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(screen.getByLabelText("Name")).toHaveAccessibleDescription("Up to 40 characters. Enter your name.");
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Enter a password.");
        const summary = screen.getByRole("alert");
        expect(summary).toHaveTextContent("Enter your name. Enter a password.");
        expect(summary).toHaveFocus();
        expect(accept.mutate).not.toHaveBeenCalled();
        expect(accept.reset).toHaveBeenCalled();
    });

    it("ties a password the server finds too short to the password field", () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "the password is too short",
            code: "password_too_short",
        });
        useAcceptInvitation.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Choose a longer password.");
        expect(screen.getByRole("alert")).toHaveTextContent("Choose a longer password.");
    });

    it("says the address already has an account when the server refuses it", () => {
        // given
        const failure = makeApiError({ status: 409, message: "that email address is already in use" });
        useAcceptInvitation.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("An account already uses this email address.");
    });

    it("shows the server's reason when the invitation stopped working while the form was open, and moves focus to it", () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "the invitation link is invalid, used or has expired",
            code: "invitation_invalid",
        });
        useAcceptInvitation.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("the invitation link is invalid, used or has expired");
        expect(alert).toHaveFocus();
    });

    it("holds the button while the account is being created", () => {
        // given
        useAcceptInvitation.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        expect(screen.getByRole("button", { name: "Create account" })).toBeDisabled();
    });

    it("links to sign in for someone who already has an account", () => {
        // given
        const route = "/accept-invitation";

        // when
        renderWithProviders(<AcceptInvitationPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });
});

describe("AcceptInvitationPage once the account is created", () => {
    it("says the account is ready and offers Sign in, without signing anyone in", () => {
        // given
        useAcceptInvitation.mockReturnValue(
            makeMutationState({ isSuccess: true, data: { email: "kit@example.test" } }),
        );

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation", user: null });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Your account is ready" })).toBeInTheDocument();
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent("Sign in to get started.");
        expect(notice).toHaveFocus();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
    });
});

describe("AcceptInvitationPage for a link that does not work", () => {
    it("shows the server's reason for a link that is invalid, used or expired, with no form and nothing to retry", () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "the invitation link is invalid, used or has expired",
            code: "invitation_invalid",
        });
        useInvitation.mockReturnValue(makeQueryState<InvitationResponse>({ error: failure }));

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("the invitation link is invalid, used or has expired");
        expect(alert).toHaveFocus();
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("offers to look the invitation up again when the server failed", async () => {
        // given
        const lookup = makeQueryState<InvitationResponse>({
            error: makeApiError({ status: 500, message: "failed to read the invitation" }),
        });
        useInvitation.mockReturnValue(lookup);
        const user = userEvent.setup();
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to read the invitation");
        expect(lookup.refetch).toHaveBeenCalledOnce();
    });

    it("says the link is missing its code when it has none, and looks nothing up", () => {
        // given
        readLocationToken.mockReturnValue("");
        useInvitation.mockReturnValue(makeQueryState<InvitationResponse>());

        // when
        renderWithProviders(<AcceptInvitationPage />, { route: "/accept-invitation" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("This link is missing its code. Open the link from your email again.");
        expect(alert).toHaveFocus();
        expect(useInvitation).toHaveBeenCalledWith("");
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
    });
});
