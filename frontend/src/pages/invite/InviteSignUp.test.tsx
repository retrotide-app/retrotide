import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { returnPathFrom } from "../../domain/returnTo";
import type { InviteDetailsResponse, SiteInfoResponse } from "../../types/api";
import {
    makeApiError,
    makeIdentityProvider,
    makeInviteDetails,
    makeInviteRegistration,
    makeJoinedOrganisation,
    makeMutationState,
    makeProviderSignIn,
    makeTurnstileControl,
    makeUpgradeRequiredError,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { InviteSignUp } from "./InviteSignUp";

const { useRegisterThroughInvite, useResendVerification, useProviderSignIn, useTurnstile } = vi.hoisted(() => ({
    useRegisterThroughInvite: vi.fn(),
    useResendVerification: vi.fn(),
    useProviderSignIn: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../hooks/mutations/invites", () => ({ useRegisterThroughInvite }));
vi.mock("../../hooks/mutations/auth", () => ({ useResendVerification }));
vi.mock("../../hooks/useProviderSignIn", () => ({ useProviderSignIn }));
vi.mock("../../hooks/useTurnstile", () => ({ useTurnstile }));

const TOKEN = "invite-token-1";
const linkInvite = makeInviteDetails({ email: undefined });
const emailInvite = makeInviteDetails({ email: "kit@example.test" });
const google = makeIdentityProvider();
const INVITE_INVALID =
    "This invite link is invalid, used, revoked or has expired. Ask the person who invited you for a new one.";

interface SignUpSetup {
    invite?: InviteDetailsResponse;
    siteInfo?: Partial<SiteInfoResponse>;
}

function SignInPage() {
    const location = useLocation();

    return <p>signing in, then back to {returnPathFrom(location.state)}</p>;
}

function renderSignUp({ invite = linkInvite, siteInfo }: SignUpSetup = {}) {
    const forget = vi.fn();

    renderWithProviders(
        <Routes>
            <Route path="/invite" element={<InviteSignUp invite={invite} token={TOKEN} forget={forget} />} />
            <Route path="/login" element={<SignInPage />} />
        </Routes>,
        { route: "/invite", siteInfo },
    );

    return { forget };
}

function answering(outcome: { registered?: unknown; failure?: Error }) {
    return makeMutationState({
        mutate: vi.fn(
            (
                _variables: unknown,
                options?: { onSuccess?: (registered: unknown) => void; onError?: (failure: Error) => void },
            ) => {
                if (outcome.failure !== undefined) {
                    options?.onError?.(outcome.failure);
                    return;
                }

                options?.onSuccess?.(outcome.registered);
            },
        ),
    });
}

function refused(error: Error) {
    return makeMutationState({ isError: true, error });
}

async function fillIn(user: ReturnType<typeof userEvent.setup>) {
    await user.type(screen.getByLabelText("Name"), " Kit ");
    await user.type(screen.getByLabelText("Email address"), " kit@example.test ");
    await user.type(screen.getByLabelText("Password"), "harbour-lights");
}

beforeEach(() => {
    useRegisterThroughInvite.mockReturnValue(makeMutationState());
    useResendVerification.mockReturnValue(makeMutationState());
    useProviderSignIn.mockReturnValue(makeProviderSignIn());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("InviteSignUp", () => {
    it("shows the invite, a way to sign in and a form that creates an account", () => {
        // given
        const invite = makeInviteDetails({ inviter_name: "Sam", org_name: "Harbour", email: undefined });

        // when
        renderSignUp({ invite });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Accept your invite" })).toBeInTheDocument();
        expect(screen.getByText(/invited you to join/)).toHaveTextContent(
            "Sam invited you to join Harbour as a member.",
        );
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        const form = screen.getByRole("form", { name: "Create an account" });
        expect(form).toContainElement(screen.getByLabelText("Name"));
        expect(form).toContainElement(screen.getByLabelText("Password"));
        expect(screen.getByLabelText("Email address")).not.toHaveAttribute("readonly");
        expect(screen.getByRole("button", { name: "Create account" })).toBeEnabled();
    });

    it("sends the user to sign in and back to the invite", async () => {
        // given
        const user = userEvent.setup();
        renderSignUp();

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText("signing in, then back to /invite")).toBeInTheDocument();
    });

    it("fixes the address of an invite sent to one", () => {
        // given
        const invite = emailInvite;

        // when
        renderSignUp({ invite });

        // then
        const email = screen.getByLabelText("Email address");
        expect(email).toHaveValue("kit@example.test");
        expect(email).toHaveAttribute("readonly");
        expect(email).toHaveAccessibleDescription("Your invite was sent to this address.");
    });

    it("stops the name at the length the server keeps, and says so", () => {
        // given
        const invite = linkInvite;

        // when
        renderSignUp({ invite });

        // then
        expect(screen.getByLabelText("Name")).toHaveAttribute("maxlength", "40");
        expect(screen.getByLabelText("Name")).toHaveAccessibleDescription("Up to 40 characters.");
    });

    it("creates the account through the invite with the details entered", async () => {
        // given
        const register = makeMutationState();
        useRegisterThroughInvite.mockReturnValue(register);
        const user = userEvent.setup();
        renderSignUp();

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(register.mutate).toHaveBeenCalledExactlyOnceWith(
            { token: TOKEN, email: "kit@example.test", display_name: "Kit", password: "harbour-lights" },
            expect.anything(),
        );
    });

    it("creates the account with the address an email invite was sent to", async () => {
        // given
        const register = makeMutationState();
        useRegisterThroughInvite.mockReturnValue(register);
        const user = userEvent.setup();
        renderSignUp({ invite: emailInvite });

        // when
        await user.type(screen.getByLabelText("Name"), "Kit");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(register.mutate).toHaveBeenCalledExactlyOnceWith(
            { token: TOKEN, email: "kit@example.test", display_name: "Kit", password: "harbour-lights" },
            expect.anything(),
        );
    });

    it("forgets the kept invite once the account has joined", async () => {
        // given
        useRegisterThroughInvite.mockReturnValue(answering({ registered: makeInviteRegistration() }));
        const user = userEvent.setup();
        const { forget } = renderSignUp();

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(forget).toHaveBeenCalledOnce();
    });

    it("says what is missing, ties each message to its field and moves focus to the summary", async () => {
        // given
        const register = makeMutationState();
        useRegisterThroughInvite.mockReturnValue(register);
        const user = userEvent.setup();
        renderSignUp();

        // when
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(screen.getByLabelText("Name")).toHaveAccessibleDescription("Up to 40 characters. Enter your name.");
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription("Enter your email address.");
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Enter a password.");
        const summary = screen.getByRole("alert");
        expect(summary).toHaveTextContent("Enter your name. Enter your email address. Enter a password.");
        expect(summary).toHaveFocus();
        expect(register.mutate).not.toHaveBeenCalled();
        expect(register.reset).toHaveBeenCalled();
    });

    it("holds the button while the account is being created", () => {
        // given
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderSignUp();

        // then
        expect(screen.getByRole("button", { name: "Create account" })).toBeDisabled();
    });
});

describe("InviteSignUp when the server refuses", () => {
    it("says the invite no longer works", () => {
        // given
        const failure = makeApiError({ status: 400, message: "the invite link is invalid", code: "invite_invalid" });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp();

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent(INVITE_INVALID);
        expect(alert).toHaveFocus();
    });

    it("forgets an invite the server calls invalid", async () => {
        // given
        const failure = makeApiError({ status: 400, message: "the invite link is invalid", code: "invite_invalid" });
        useRegisterThroughInvite.mockReturnValue(answering({ failure }));
        const user = userEvent.setup();
        const { forget } = renderSignUp();

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(forget).toHaveBeenCalledOnce();
    });

    it("keeps the invite after a refusal that leaves it usable", async () => {
        // given
        const failure = makeApiError({ status: 400, message: "the password is too short", code: "password_too_short" });
        useRegisterThroughInvite.mockReturnValue(answering({ failure }));
        const user = userEvent.setup();
        const { forget } = renderSignUp();

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(forget).not.toHaveBeenCalled();
    });

    it("ties a password the server finds too short to the password field", () => {
        // given
        const failure = makeApiError({ status: 400, message: "the password is too short", code: "password_too_short" });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp();

        // then
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Choose a longer password.");
        expect(screen.getByRole("alert")).toHaveTextContent("Choose a longer password.");
    });

    it("ties an address that already has an account to the email field and says to sign in", () => {
        // given
        const failure = makeApiError({
            status: 409,
            message: "that email address already has an account, so sign in to accept the invite",
            code: "email_taken",
        });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp({ invite: emailInvite });

        // then
        const message = "An account already uses this email address. Sign in to accept the invite.";
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription(
            `Your invite was sent to this address. ${message}`,
        );
        expect(screen.getByRole("alert")).toHaveTextContent(message);
    });

    it("blames the address only for the refusal coded as a taken address", () => {
        // given
        const failure = makeApiError({ status: 409, message: "a conflict the address does not own" });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp({ invite: emailInvite });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("a conflict the address does not own");
        expect(screen.getByLabelText("Email address")).not.toBeInvalid();
    });

    it("ties a name the server refuses to the name field", () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "a display name is required",
            code: "display_name_required",
        });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp();

        // then
        expect(screen.getByLabelText("Name")).toHaveAccessibleDescription(
            expect.stringContaining("a display name is required"),
        );
        expect(screen.getByRole("alert")).toHaveTextContent("a display name is required");
    });

    it("says the invite is for a different address", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "this invite is for a different email address",
            code: "invite_email_mismatch",
        });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp({ invite: emailInvite });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("This invite is for a different email address.");
    });

    it("says paid plans are coming when the organisation is at its member limit", () => {
        // given
        const failure = makeUpgradeRequiredError({ limit: "members", allowed: 10, current: 10 });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp({ invite: makeInviteDetails({ org_name: "Harbour" }) });

        // then
        const alerts = screen.getAllByRole("alert");
        expect(alerts).toHaveLength(1);
        expect(alerts[0]).toHaveTextContent("The free plan allows 10 members.");
        expect(alerts[0]).toHaveTextContent("Paid plans are coming. Until then, this limit stays.");
        expect(alerts[0]).toHaveFocus();
        expect(screen.getByText(/has reached its plan's limit/)).toHaveTextContent(
            "Harbour has reached its plan's limit. Ask the person who invited you.",
        );
    });

    it("says a link invite cannot confirm a new address while the site cannot send email", () => {
        // given
        const failure = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This site cannot send email yet, so it cannot confirm a new address. Ask whoever invited you to send the invite to your email address instead.",
        );
    });

    it("shows the server's message for any other refusal", () => {
        // given
        const failure = makeApiError({ status: 503, message: "email is not available on this site" });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("email is not available on this site");
    });

    it("says creating the account failed when the refusal carries no message", () => {
        // given
        const failure = makeApiError({ status: 500, message: "" });
        useRegisterThroughInvite.mockReturnValue(refused(failure));

        // when
        renderSignUp();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Creating your account failed. Try again.");
    });
});

describe("InviteSignUp once the account is created", () => {
    it("says the account is ready and has joined, and signs in straight to the organisation", async () => {
        // given
        const registered = makeInviteRegistration({
            verification_required: false,
            joined: makeJoinedOrganisation({ org_name: "Harbour", org_slug: "harbour", team_id: undefined }),
        });
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isSuccess: true, data: registered }));
        const user = userEvent.setup();
        renderSignUp();

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText("signing in, then back to /o/harbour")).toBeInTheDocument();
    });

    it("tells a verified account it is ready and moves focus to the notice", () => {
        // given
        const registered = makeInviteRegistration({
            verification_required: false,
            joined: makeJoinedOrganisation({ org_name: "Harbour" }),
        });
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isSuccess: true, data: registered }));

        // when
        renderSignUp();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Your account is ready" })).toBeInTheDocument();
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent("You joined Harbour. Sign in to get started.");
        expect(notice).toHaveFocus();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
    });

    it("signs in straight to the team the invite joined", async () => {
        // given
        const registered = makeInviteRegistration({
            verification_required: false,
            joined: makeJoinedOrganisation({ org_slug: "harbour", team_id: "team-3" }),
        });
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isSuccess: true, data: registered }));
        const user = userEvent.setup();
        renderSignUp();

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText("signing in, then back to /o/harbour/teams/team-3")).toBeInTheDocument();
    });

    it("signs a verified address in straight to what the invite joined", async () => {
        // given
        const registered = makeInviteRegistration({
            verification_required: true,
            joined: makeJoinedOrganisation({ org_slug: "harbour", team_id: "team-3" }),
        });
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isSuccess: true, data: registered }));
        const user = userEvent.setup();
        renderSignUp();

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText("signing in, then back to /o/harbour/teams/team-3")).toBeInTheDocument();
    });

    it("asks an account whose address needs verifying to check its inbox", () => {
        // given
        const registered = makeInviteRegistration({
            email: "kit@example.test",
            verification_required: true,
            joined: makeJoinedOrganisation({ org_name: "Harbour" }),
        });
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isSuccess: true, data: registered }));

        // when
        renderSignUp();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Check your inbox" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent(
            "You joined Harbour. We sent a link to kit@example.test. Open it to verify your email address, then sign in.",
        );
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("sends the verification link again to the new account's address", async () => {
        // given
        const registered = makeInviteRegistration({ email: "kit@example.test", verification_required: true });
        useRegisterThroughInvite.mockReturnValue(makeMutationState({ isSuccess: true, data: registered }));
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderSignUp();

        // when
        await user.click(screen.getByRole("button", { name: "Resend link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "kit@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });
});

describe("InviteSignUp with sign-in providers", () => {
    it("offers no provider sign-up while the site has none switched on", () => {
        // given
        const siteInfo = { identity_providers: [] };

        // when
        renderSignUp({ siteInfo });

        // then
        expect(screen.queryByRole("button", { name: /^Continue with/ })).not.toBeInTheDocument();
    });

    it("starts a provider sign-up that comes back to the invite", async () => {
        // given
        const register = makeMutationState();
        useRegisterThroughInvite.mockReturnValue(register);
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderSignUp({ siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(google, { purpose: "sign_in", return_to: "/invite" });
        expect(register.reset).toHaveBeenCalledOnce();
    });

    it("clears a provider's failure when the form is sent instead", async () => {
        // given
        const signIn = makeProviderSignIn({
            provider: google,
            error: makeApiError({ status: 429, message: "too many requests" }),
        });
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderSignUp({ siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(signIn.reset).toHaveBeenCalledOnce();
    });
});

describe("InviteSignUp while password sign-in is off", () => {
    const providersOnly = { password_sign_in: false, identity_providers: [google] };

    it("draws no password form and points the visitor to a provider", () => {
        // given
        const siteInfo = providersOnly;

        // when
        renderSignUp({ siteInfo });

        // then
        const lead = "New here? Continue with a sign-in provider to create an account and join.";
        expect(screen.getByText(lead)).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.queryByRole("form", { name: "Create an account" })).not.toBeInTheDocument();
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
        expect(screen.queryByText("or")).not.toBeInTheDocument();
    });

    it("still offers sign in for an existing account, back to the invite", async () => {
        // given
        const user = userEvent.setup();
        renderSignUp({ siteInfo: providersOnly });

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText("signing in, then back to /invite")).toBeInTheDocument();
    });

    it("starts a provider sign-up that comes back to the invite", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderSignUp({ siteInfo: providersOnly });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(google, { purpose: "sign_in", return_to: "/invite" });
    });
});
