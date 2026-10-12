import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeInviteDetails,
    makeMutationState,
    makeProviderSignIn,
    makeQueryState,
    makeSessionUser,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { InviteAcceptancePage } from "./InviteAcceptancePage";

const {
    useInviteToken,
    useInviteLookup,
    useAcceptInvite,
    useRegisterThroughInvite,
    useLogout,
    useResendVerification,
    useProviderSignIn,
} = vi.hoisted(() => ({
    useInviteToken: vi.fn(),
    useInviteLookup: vi.fn(),
    useAcceptInvite: vi.fn(),
    useRegisterThroughInvite: vi.fn(),
    useLogout: vi.fn(),
    useResendVerification: vi.fn(),
    useProviderSignIn: vi.fn(),
}));

vi.mock("../../hooks/useInviteToken", () => ({ useInviteToken }));
vi.mock("../../hooks/queries/invites", () => ({ useInviteLookup }));
vi.mock("../../hooks/mutations/invites", () => ({ useAcceptInvite, useRegisterThroughInvite }));
vi.mock("../../hooks/mutations/auth", () => ({ useLogout, useResendVerification }));
vi.mock("../../hooks/useProviderSignIn", () => ({ useProviderSignIn }));

const TOKEN = "invite-token-1";
const INVITE_INVALID =
    "This invite link is invalid, used, revoked or has expired. Ask the person who invited you for a new one.";

function keptToken(token: string) {
    const forget = vi.fn();
    useInviteToken.mockReturnValue({ token, forget });

    return forget;
}

beforeEach(() => {
    keptToken(TOKEN);
    useInviteLookup.mockReturnValue(makeQueryState({ data: makeInviteDetails() }));
    useAcceptInvite.mockReturnValue(makeMutationState());
    useRegisterThroughInvite.mockReturnValue(makeMutationState());
    useLogout.mockReturnValue(makeMutationState());
    useResendVerification.mockReturnValue(makeMutationState());
    useProviderSignIn.mockReturnValue(makeProviderSignIn());
});

describe("InviteAcceptancePage", () => {
    it("looks up the invite the link carried and names the page after it", () => {
        // given
        keptToken(TOKEN);

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite" });

        // then
        expect(useInviteLookup).toHaveBeenCalledWith(TOKEN);
        expect(screen.getByRole("heading", { level: 1, name: "Accept your invite" })).toBeInTheDocument();
        expect(document.title).toContain("Accept your invite");
    });

    it("says the link is missing its code when it carries none", () => {
        // given
        keptToken("");
        useInviteLookup.mockReturnValue(makeQueryState());

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("This link is missing its code. Open the invite link again.");
        expect(alert).toHaveFocus();
        expect(screen.getByRole("link", { name: "Go to the home page" })).toHaveAttribute("href", "/");
    });

    it("says the invite is being checked while it is looked up", () => {
        // given
        useInviteLookup.mockReturnValue(makeQueryState({ loading: true }));

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite" });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Checking your invite...");
    });

    it("says an invalid invite no longer works and forgets it", () => {
        // given
        const forget = keptToken(TOKEN);
        const failure = makeApiError({
            status: 400,
            message: "the invite link is invalid, used, revoked or has expired",
            code: "invite_invalid",
        });
        useInviteLookup.mockReturnValue(makeQueryState({ error: failure }));

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent(INVITE_INVALID);
        expect(alert).toHaveFocus();
        expect(screen.getByRole("link", { name: "Go to the home page" })).toHaveAttribute("href", "/");
        expect(forget).toHaveBeenCalledOnce();
    });

    it("shows why the lookup failed and tries again, keeping the invite", async () => {
        // given
        const forget = keptToken(TOKEN);
        const lookup = makeQueryState({ error: makeApiError({ status: 500, message: "failed to read the invite" }) });
        useInviteLookup.mockReturnValue(lookup);
        const user = userEvent.setup();
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite" });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to read the invite");
        expect(lookup.refetch).toHaveBeenCalledOnce();
        expect(forget).not.toHaveBeenCalled();
    });

    it("says the invite could not be read when the failure carries no message", () => {
        // given
        useInviteLookup.mockReturnValue(makeQueryState({ error: makeApiError({ status: 502, message: "" }) }));

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Your invite could not be read.");
    });

    it("keeps showing the invite when a later check of it fails", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to read the invite" });
        useInviteLookup.mockReturnValue(makeQueryState({ data: makeInviteDetails(), error: failure }));

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite", user: makeSessionUser() });

        // then
        expect(screen.getByRole("button", { name: "Accept invite" })).toBeInTheDocument();
    });

    it("offers a signed-in user the button that accepts the invite", () => {
        // given
        const user = makeSessionUser({ email: "sam@example.test" });

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite", user });

        // then
        expect(screen.getByRole("button", { name: "Accept invite" })).toBeInTheDocument();
        expect(screen.getByText(/You are signed in as/)).toHaveTextContent("You are signed in as sam@example.test.");
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
    });

    it("offers a signed-out visitor sign in or a new account", () => {
        // given
        const user = null;

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite", user });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Accept invite" })).not.toBeInTheDocument();
    });

    it("offers sign in or a new account on a private site too", () => {
        // given
        const siteInfo = { site_mode: "private" as const, registration_open: false };

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite", siteInfo });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
    });

    it("says the sign-in is being checked while the session loads", () => {
        // given
        const auth = { loading: true };

        // when
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite", auth });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Checking your sign-in...");
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
    });

    it("shows why the sign-in check failed and tries again", async () => {
        // given
        const retry = vi.fn().mockResolvedValue(undefined);
        const auth = { error: new Error("failed to read the session"), retry };
        const user = userEvent.setup();
        renderWithProviders(<InviteAcceptancePage />, { route: "/invite", auth });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("heading", { level: 2, name: "We could not check your sign-in" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("failed to read the session");
        expect(retry).toHaveBeenCalledOnce();
    });
});
