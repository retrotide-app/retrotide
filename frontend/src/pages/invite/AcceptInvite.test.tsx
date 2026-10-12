import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { InviteDetailsResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";
import {
    makeApiError,
    makeInviteDetails,
    makeJoinedOrganisation,
    makeMutationState,
    makeSessionUser,
    makeUpgradeRequiredError,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { AcceptInvite } from "./AcceptInvite";

const { useAcceptInvite, useLogout } = vi.hoisted(() => ({
    useAcceptInvite: vi.fn(),
    useLogout: vi.fn(),
}));

vi.mock("../../hooks/mutations/invites", () => ({ useAcceptInvite }));
vi.mock("../../hooks/mutations/auth", () => ({ useLogout }));

const TOKEN = "invite-token-1";
const INVITE_INVALID =
    "This invite link is invalid, used, revoked or has expired. Ask the person who invited you for a new one.";
const EMAIL_MISMATCH =
    "This invite is for a different email address. Sign out, then sign in with the address it was sent to.";

interface AcceptSetup {
    invite?: InviteDetailsResponse;
    user?: SessionUser;
}

function Landed() {
    const location = useLocation();

    return <p>landed on {location.pathname}</p>;
}

function renderAccept({ invite = makeInviteDetails(), user = makeSessionUser() }: AcceptSetup = {}) {
    const forget = vi.fn();
    const selectOrganisation = vi.fn();

    renderWithProviders(
        <Routes>
            <Route
                path="/invite"
                element={<AcceptInvite invite={invite} token={TOKEN} user={user} forget={forget} />}
            />
            <Route path="*" element={<Landed />} />
        </Routes>,
        { route: "/invite", user, org: { selectOrganisation } },
    );

    return { forget, selectOrganisation };
}

function answering(outcome: { joined?: unknown; failure?: Error }) {
    return makeMutationState({
        mutate: vi.fn(
            (
                _variables: unknown,
                options?: { onSuccess?: (joined: unknown) => void; onError?: (failure: Error) => void },
            ) => {
                if (outcome.failure !== undefined) {
                    options?.onError?.(outcome.failure);
                    return;
                }

                options?.onSuccess?.(outcome.joined);
            },
        ),
    });
}

function refused(error: Error) {
    return makeMutationState({ isError: true, error });
}

beforeEach(() => {
    useAcceptInvite.mockReturnValue(makeMutationState());
    useLogout.mockReturnValue(makeMutationState());
});

describe("AcceptInvite", () => {
    it("shows the invite, the account that will join and the button that accepts it", () => {
        // given
        const invite = makeInviteDetails({ inviter_name: "Sam", org_name: "Harbour" });

        // when
        renderAccept({ invite, user: makeSessionUser({ email: "kit@example.test" }) });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Accept your invite" })).toBeInTheDocument();
        expect(screen.getByText(/invited you to join/)).toHaveTextContent(
            "Sam invited you to join Harbour as a member.",
        );
        expect(screen.getByText(/You are signed in as/)).toHaveTextContent("You are signed in as kit@example.test.");
        expect(screen.getByRole("button", { name: "Accept invite" })).toBeEnabled();
        expect(screen.queryByRole("button", { name: "Sign out" })).not.toBeInTheDocument();
    });

    it("accepts the invite with the token the link carried", async () => {
        // given
        const accept = makeMutationState();
        useAcceptInvite.mockReturnValue(accept);
        const user = userEvent.setup();
        renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Accept invite" }));

        // then
        expect(accept.mutate).toHaveBeenCalledExactlyOnceWith({ token: TOKEN }, expect.anything());
    });

    it("forgets the invite, makes the organisation current and opens the team the invite joined", async () => {
        // given
        const joined = makeJoinedOrganisation({ org_id: "org-7", org_slug: "harbour", team_id: "team-3" });
        useAcceptInvite.mockReturnValue(answering({ joined }));
        const user = userEvent.setup();
        const { forget, selectOrganisation } = renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Accept invite" }));

        // then
        expect(forget).toHaveBeenCalledOnce();
        expect(selectOrganisation).toHaveBeenCalledExactlyOnceWith("org-7");
        expect(screen.getByText("landed on /o/harbour/teams/team-3")).toBeInTheDocument();
    });

    it("opens the organisation's boards when the invite joins no team", async () => {
        // given
        const joined = makeJoinedOrganisation({ org_slug: "harbour", team_id: undefined });
        useAcceptInvite.mockReturnValue(answering({ joined }));
        const user = userEvent.setup();
        renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Accept invite" }));

        // then
        expect(screen.getByText("landed on /o/harbour")).toBeInTheDocument();
    });

    it("holds the button while the invite is being accepted", () => {
        // given
        useAcceptInvite.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderAccept();

        // then
        expect(screen.getByRole("button", { name: "Accept invite" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Not now" })).toBeDisabled();
    });
});

describe("AcceptInvite declining for now", () => {
    it("forgets the kept invite and goes on to the app, without accepting", async () => {
        // given
        const accept = makeMutationState();
        useAcceptInvite.mockReturnValue(accept);
        const user = userEvent.setup();
        const { forget, selectOrganisation } = renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Not now" }));

        // then
        expect(forget).toHaveBeenCalledOnce();
        expect(accept.mutate).not.toHaveBeenCalled();
        expect(selectOrganisation).not.toHaveBeenCalled();
        expect(screen.getByText("landed on /")).toBeInTheDocument();
    });

    it("offers it beside signing out when the invite is for another address", () => {
        // when
        renderAccept({
            invite: makeInviteDetails({ email: "kit@example.test" }),
            user: makeSessionUser({ email: "sam@example.test" }),
        });

        // then
        expect(screen.getByRole("button", { name: "Sign out" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Not now" })).toBeEnabled();
    });

    it.each([
        { status: 400, message: "the invite link is invalid", code: "invite_invalid" },
        { status: 409, message: "already a member of this organisation", code: "already_member" },
    ])("offers nothing to decline once the invite is settled: $code", refusal => {
        // given
        useAcceptInvite.mockReturnValue(refused(makeApiError(refusal)));

        // when
        renderAccept();

        // then
        expect(screen.queryByRole("button", { name: "Not now" })).not.toBeInTheDocument();
    });
});

describe("AcceptInvite when the server refuses", () => {
    it("says the invite no longer works and offers the home page", () => {
        // given
        const failure = makeApiError({ status: 400, message: "the invite link is invalid", code: "invite_invalid" });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept();

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent(INVITE_INVALID);
        expect(alert).toHaveFocus();
        expect(screen.getByRole("link", { name: "Go to the home page" })).toHaveAttribute("href", "/");
        expect(screen.queryByRole("button", { name: "Accept invite" })).not.toBeInTheDocument();
    });

    it("forgets an invite the server calls invalid", async () => {
        // given
        const failure = makeApiError({ status: 400, message: "the invite link is invalid", code: "invite_invalid" });
        useAcceptInvite.mockReturnValue(answering({ failure }));
        const user = userEvent.setup();
        const { forget, selectOrganisation } = renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Accept invite" }));

        // then
        expect(forget).toHaveBeenCalledOnce();
        expect(selectOrganisation).not.toHaveBeenCalled();
    });

    it("says the invite is for another address and offers to sign out", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "this invite is for a different email address",
            code: "invite_email_mismatch",
        });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(EMAIL_MISMATCH);
        expect(screen.getByRole("button", { name: "Sign out" })).toBeEnabled();
        expect(screen.queryByRole("button", { name: "Accept invite" })).not.toBeInTheDocument();
    });

    it("keeps the invite when the address does not match, so the right account can accept it", async () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "this invite is for a different email address",
            code: "invite_email_mismatch",
        });
        useAcceptInvite.mockReturnValue(answering({ failure }));
        const user = userEvent.setup();
        const { forget } = renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Accept invite" }));

        // then
        expect(forget).not.toHaveBeenCalled();
    });

    it("asks for a verified address and lets the user try again", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "verify your email address before accepting this invite",
            code: "email_unverified",
        });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Verify your email address before you accept this invite.");
        expect(screen.getByRole("button", { name: "Accept invite" })).toBeEnabled();
    });

    it("says the user is already a member and links to the organisation", () => {
        // given
        const failure = makeApiError({
            status: 409,
            message: "already a member of this organisation",
            code: "already_member",
        });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept({ invite: makeInviteDetails({ org_name: "Harbour", org_slug: "harbour" }) });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("You are already a member of Harbour.");
        expect(screen.getByRole("link", { name: "Go to Harbour" })).toHaveAttribute("href", "/o/harbour");
        expect(screen.queryByRole("button", { name: "Accept invite" })).not.toBeInTheDocument();
    });

    it("forgets the invite of an organisation the user already belongs to", async () => {
        // given
        const failure = makeApiError({
            status: 409,
            message: "already a member of this organisation",
            code: "already_member",
        });
        useAcceptInvite.mockReturnValue(answering({ failure }));
        const user = userEvent.setup();
        const { forget } = renderAccept();

        // when
        await user.click(screen.getByRole("button", { name: "Accept invite" }));

        // then
        expect(forget).toHaveBeenCalledOnce();
    });

    it("says paid plans are coming when the organisation is at its member limit", () => {
        // given
        const failure = makeUpgradeRequiredError({ limit: "members", allowed: 10, current: 10 });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept({ invite: makeInviteDetails({ org_name: "Harbour" }) });

        // then
        const alerts = screen.getAllByRole("alert");
        expect(alerts).toHaveLength(1);
        expect(alerts[0]).toHaveTextContent("The free plan allows 10 members.");
        expect(alerts[0]).toHaveTextContent("Paid plans are coming. Until then, this limit stays.");
        expect(screen.getByText(/has reached its plan's limit/)).toHaveTextContent(
            "Harbour has reached its plan's limit. Ask the person who invited you.",
        );
    });

    it("shows the server's message for any other refusal", () => {
        // given
        const failure = makeApiError({ status: 503, message: "the service is unavailable" });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept();

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("the service is unavailable");
        expect(alert).toHaveFocus();
    });

    it("says accepting failed when the refusal carries no message", () => {
        // given
        const failure = makeApiError({ status: 500, message: "" });
        useAcceptInvite.mockReturnValue(refused(failure));

        // when
        renderAccept();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Accepting the invite failed. Try again.");
    });
});

describe("AcceptInvite for an invite sent to another address", () => {
    const invite = makeInviteDetails({ email: "kit@example.test" });

    it("names both addresses and offers to sign out instead of accepting", () => {
        // given
        const user = makeSessionUser({ email: "sam@example.test" });

        // when
        renderAccept({ invite, user });

        // then
        expect(screen.getByText(/You are signed in as/)).toHaveTextContent(
            "You are signed in as sam@example.test. This invite is for kit@example.test. Sign out, then sign in with that address to accept it.",
        );
        expect(screen.getByRole("button", { name: "Sign out" })).toBeEnabled();
        expect(screen.queryByRole("button", { name: "Accept invite" })).not.toBeInTheDocument();
    });

    it("signs out when asked", async () => {
        // given
        const accept = makeMutationState();
        useAcceptInvite.mockReturnValue(accept);
        const logout = makeMutationState();
        useLogout.mockReturnValue(logout);
        const user = userEvent.setup();
        renderAccept({ invite, user: makeSessionUser({ email: "sam@example.test" }) });

        // when
        await user.click(screen.getByRole("button", { name: "Sign out" }));

        // then
        expect(logout.mutate).toHaveBeenCalledOnce();
        expect(accept.reset).toHaveBeenCalledOnce();
    });

    it("holds the button while signing out", () => {
        // given
        useLogout.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderAccept({ invite, user: makeSessionUser({ email: "sam@example.test" }) });

        // then
        expect(screen.getByRole("button", { name: "Sign out" })).toBeDisabled();
    });

    it("shows why signing out failed", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to end the session" });
        useLogout.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderAccept({ invite, user: makeSessionUser({ email: "sam@example.test" }) });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("failed to end the session");
        expect(alert).toHaveFocus();
    });

    it("lets the account accept when only the letter case of the address differs", () => {
        // given
        const user = makeSessionUser({ email: "Kit@Example.test" });

        // when
        renderAccept({ invite, user });

        // then
        expect(screen.getByRole("button", { name: "Accept invite" })).toBeEnabled();
        expect(screen.queryByRole("button", { name: "Sign out" })).not.toBeInTheDocument();
    });
});
