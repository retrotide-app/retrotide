import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Navigate, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { type ProviderCallbackCode, providerCallbackMessage } from "../../../domain/providerCallback";
import {
    makeApiError,
    makeIdentityProvider,
    makeMutationState,
    makeProviderSignIn,
    makeSessionUser,
    makeTurnstileControl,
} from "../../../test-utils/fixtures";
import { type ProviderOptions, renderWithProviders } from "../../../test-utils/render";
import { LoginPage } from "./LoginPage";

const { useLogin, useResendVerification, useProviderSignIn, useTurnstile } = vi.hoisted(() => ({
    useLogin: vi.fn(),
    useResendVerification: vi.fn(),
    useProviderSignIn: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../../hooks/mutations/auth", () => ({ useLogin, useResendVerification }));
vi.mock("../../../hooks/useProviderSignIn", () => ({ useProviderSignIn }));
vi.mock("../../../hooks/useTurnstile", () => ({ useTurnstile }));

const CALLBACK_CODES: ProviderCallbackCode[] = [
    "cancelled",
    "expired",
    "email_unverified",
    "email_not_authoritative",
    "link_required",
    "identity_in_use",
    "already_linked",
    "account_banned",
    "account_unverified",
    "account_not_allowed",
    "failed",
];

const google = makeIdentityProvider();
const example = makeIdentityProvider({ id: "example", name: "Example ID" });

function AccountProbe() {
    const location = useLocation();

    return <p>{`the account page${location.search}`}</p>;
}

function TwoFactorProbe() {
    const location = useLocation();

    return <p>{`the two-factor page ${JSON.stringify(location.state)}`}</p>;
}

function settlingMutation() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

function refusedUnverified() {
    return makeMutationState({
        isError: true,
        error: makeApiError({
            status: 403,
            message: "verify your email address before signing in",
            code: "email_unverified",
        }),
        variables: { email: "sam@example.test", password: "harbour-lights" },
    });
}

function renderFromProtectedPage(options: Parameters<typeof renderWithProviders>[1] = {}) {
    return renderWithProviders(
        <Routes>
            <Route path="/start" element={<Navigate to="/login" state={{ from: "/account" }} />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/account" element={<p>the account page</p>} />
        </Routes>,
        { ...options, route: "/start" },
    );
}

beforeEach(() => {
    useLogin.mockReturnValue(makeMutationState());
    useResendVerification.mockReturnValue(makeMutationState());
    useProviderSignIn.mockReturnValue(makeProviderSignIn());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("LoginPage", () => {
    it("asks for an email address and a password", () => {
        // given
        const route = "/login";

        // when
        renderWithProviders(<LoginPage />, { route });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
        expect(screen.getByLabelText("Email address")).toHaveAttribute("type", "email");
        expect(screen.getByLabelText("Password")).toHaveAttribute("type", "password");
        expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });

    it("signs in with the details entered, without the spaces around the address", async () => {
        // given
        const login = makeMutationState();
        useLogin.mockReturnValue(login);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login" });

        // when
        await user.type(screen.getByLabelText("Email address"), " sam@example.test ");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then the sign-in is not remembered unless asked
        expect(login.mutate).toHaveBeenCalledOnce();
        expect(login.mutate).toHaveBeenCalledWith(
            { email: "sam@example.test", password: "harbour-lights", remember: false, turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("remembers the sign-in on this device when asked", async () => {
        // given
        const login = makeMutationState();
        useLogin.mockReturnValue(login);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login" });
        const remember = screen.getByRole("switch", { name: "Remember me" });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(remember);
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(remember).toHaveAttribute("aria-checked", "true");
        expect(login.mutate).toHaveBeenCalledWith(
            { email: "sam@example.test", password: "harbour-lights", remember: true, turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("says what is missing, ties each message to its field and moves focus to the summary", async () => {
        // given
        const login = makeMutationState();
        useLogin.mockReturnValue(login);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login" });

        // when
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription("Enter your email address.");
        expect(screen.getByLabelText("Email address")).toBeInvalid();
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Enter your password.");
        const summary = screen.getByRole("alert");
        expect(summary).toHaveTextContent("Enter your email address. Enter your password.");
        expect(summary).toHaveFocus();
        expect(login.mutate).not.toHaveBeenCalled();
        expect(login.reset).toHaveBeenCalled();
    });

    it("shows the server's reason when sign-in fails and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 401, message: "invalid email or password" });
        useLogin.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("invalid email or password");
        expect(alert).toHaveFocus();
    });

    it("shows the ban when a banned account tries to sign in", () => {
        // given
        const failure = makeApiError({ status: 403, message: "your account has been banned" });
        useLogin.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("your account has been banned");
    });

    it("holds the button while signing in", () => {
        // given
        useLogin.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.getByRole("button", { name: "Sign in" })).toBeDisabled();
    });

    it("sends a signed-in visitor to the home page", () => {
        // given
        const ui = (
            <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/" element={<p>the home page</p>} />
            </Routes>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser(), route: "/login" });

        // then
        expect(screen.getByText("the home page")).toBeInTheDocument();
    });

    it("sends a signed-in visitor back to the page that asked them to sign in", () => {
        // given
        const user = makeSessionUser();

        // when
        renderFromProtectedPage({ user });

        // then
        expect(screen.getByText("the account page")).toBeInTheDocument();
    });

    it("says why the sign-in did not stick when the session could not be read afterwards", () => {
        // given
        useLogin.mockReturnValue(makeMutationState({ isSuccess: true, data: makeSessionUser() }));
        const auth = { error: makeApiError({ status: 500, message: "failed to load the session" }) };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", auth });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to load the session");
    });

    it("says the browser did not keep the sign-in when the session still reads as signed out", () => {
        // given
        useLogin.mockReturnValue(makeMutationState({ isSuccess: true, data: makeSessionUser() }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "You signed in, but this browser did not keep you signed in. Allow cookies for this site, then try again.",
        );
    });

    it("says nothing about cookies while a second factor is still owed", () => {
        // given
        useLogin.mockReturnValue(makeMutationState({ isSuccess: true, data: { two_factor_required: true } }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("goes on to the two-factor step, carrying the page that asked the visitor to sign in", async () => {
        // given
        useLogin.mockReturnValue(
            makeMutationState({
                mutate: vi.fn((_variables: unknown, options?: { onSuccess?: (response: unknown) => void }) => {
                    options?.onSuccess?.({ two_factor_required: true });
                }),
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route path="/start" element={<Navigate to="/login" state={{ from: "/account" }} />} />
                <Route path="/login" element={<LoginPage />} />
                <Route path="/two-factor" element={<TwoFactorProbe />} />
            </Routes>,
            { route: "/start" },
        );

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(screen.getByText('the two-factor page {"from":"/account"}')).toBeInTheDocument();
    });

    it("stays on the sign-in page when the password alone signs the visitor in", async () => {
        // given
        useLogin.mockReturnValue(
            makeMutationState({
                mutate: vi.fn((_variables: unknown, options?: { onSuccess?: (response: unknown) => void }) => {
                    options?.onSuccess?.(makeSessionUser());
                }),
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/two-factor" element={<TwoFactorProbe />} />
            </Routes>,
            { route: "/login" },
        );

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then the session takes the visitor on once it reads as signed in
        expect(screen.queryByText(/the two-factor page/)).not.toBeInTheDocument();
        expect(screen.getByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
    });

    it("says password sign-in is turned off when the server refuses it", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "password sign-in is turned off on this site",
            code: "password_sign_in_disabled",
        });
        useLogin.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Password sign-in is turned off on this site.");
    });

    it("offers a way to reset a forgotten password", () => {
        // given
        const route = "/login";

        // when
        renderWithProviders(<LoginPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Forgotten your password?" })).toHaveAttribute(
            "href",
            "/forgot-password",
        );
    });

    it("offers to create an account on a public site", () => {
        // given
        const siteInfo = { site_mode: "public" as const, registration_open: true };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.getByRole("link", { name: "Create an account" })).toHaveAttribute("href", "/register");
    });

    it("offers to create the first account on a private site that has none yet", () => {
        // given
        const siteInfo = { site_mode: "private" as const, registration_open: true };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.getByRole("link", { name: "Create an account" })).toHaveAttribute("href", "/register");
    });

    it("offers no account creation on a private site once it has an account", () => {
        // given
        const siteInfo = { site_mode: "private" as const, registration_open: false };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.queryByRole("link", { name: "Create an account" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Forgotten your password?" })).toHaveAttribute(
            "href",
            "/forgot-password",
        );
    });

    it("still offers account creation on a public site that cannot send email but has a provider", () => {
        // given
        const siteInfo = {
            site_mode: "public" as const,
            registration_open: false,
            email_enabled: false,
            identity_providers: [google],
        };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then a provider sign-up needs no email
        expect(screen.getByRole("link", { name: "Create an account" })).toHaveAttribute("href", "/register");
        expect(screen.queryByText("New accounts are closed until this site can send email.")).not.toBeInTheDocument();
    });

    it("shows account creation as unavailable, and says why, on a public site that cannot send email and has no provider", () => {
        // given
        const siteInfo = { site_mode: "public" as const, registration_open: false, email_enabled: false };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.queryByRole("link", { name: "Create an account" })).not.toBeInTheDocument();
        expect(screen.getByText("Create an account")).toHaveAttribute("aria-disabled", "true");
        expect(screen.getByText("New accounts are closed until this site can send email.")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });

    it("offers only the password form on a private site, which lists no sign-in providers", () => {
        // given
        const siteInfo = { site_mode: "private" as const, registration_open: false, identity_providers: [] };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.queryByRole("button", { name: /^Continue with/ })).not.toBeInTheDocument();
        expect(screen.getByLabelText("Password")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });
});

describe("LoginPage for an account whose address is not verified yet", () => {
    it("says to verify the address first, and moves focus to it", () => {
        // given
        useLogin.mockReturnValue(refusedUnverified());

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("Verify your email address before you sign in.");
        expect(alert).toHaveFocus();
    });

    it("sends the link again to the address that was refused, with no session", async () => {
        // given
        useLogin.mockReturnValue(refusedUnverified());
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login", user: null });

        // when
        await user.click(screen.getByRole("button", { name: "Resend link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("says the link was sent, and keeps the form for signing in once the address is verified", () => {
        // given
        useLogin.mockReturnValue(refusedUnverified());
        useResendVerification.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Link sent to sam@example.test. Check your inbox.");
        expect(screen.queryByRole("button", { name: "Resend link" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });

    it.each([
        { status: 401, message: "invalid email or password" },
        { status: 403, message: "your account has been banned" },
    ])("offers no new link for any other refusal: $message", refusal => {
        // given
        useLogin.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError(refusal),
                variables: { email: "sam@example.test", password: "harbour-lights" },
            }),
        );

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(screen.queryByRole("button", { name: "Resend link" })).not.toBeInTheDocument();
    });
});

describe("LoginPage with Turnstile", () => {
    it("draws no verification check while the site has it switched off", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: false });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(turnstile.attach).not.toHaveBeenCalled();
    });

    it("draws the verification check while the site has it switched on", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: true });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<LoginPage />, { route: "/login" });

        // then
        expect(turnstile.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("asks for the verification check before sending anything", async () => {
        // given
        const login = makeMutationState();
        useLogin.mockReturnValue(login);
        useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login" });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Complete the verification check.");
        expect(login.mutate).not.toHaveBeenCalled();
    });

    it("sends the verification token with the sign-in, then resets the check for the next try", async () => {
        // given
        const login = settlingMutation();
        useLogin.mockReturnValue(login);
        const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-1" });
        useTurnstile.mockReturnValue(turnstile);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login" });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(login.mutate).toHaveBeenCalledWith(
            { email: "sam@example.test", password: "harbour-lights", remember: false, turnstile_token: "turnstile-1" },
            expect.anything(),
        );
        expect(turnstile.reset).toHaveBeenCalledOnce();
    });
});

describe("LoginPage with sign-in providers", () => {
    it("offers no provider sign-in while the site has none switched on", () => {
        // given
        const siteInfo = { identity_providers: [] };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.queryByRole("button", { name: /^Continue with/ })).not.toBeInTheDocument();
        expect(screen.queryByText("or")).not.toBeInTheDocument();
    });

    it("offers one button for each provider the site has switched on", () => {
        // given
        const siteInfo = { identity_providers: [google, example] };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Example ID" })).toBeInTheDocument();
    });

    it("offers only the providers while password sign-in is turned off", () => {
        // given
        const siteInfo = { password_sign_in: false, identity_providers: [google, example] };

        // when
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // then there is no password form, no reset link and nothing to choose between
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Example ID" })).toBeInTheDocument();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Sign in" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Forgotten your password?" })).not.toBeInTheDocument();
        expect(screen.queryByText("or")).not.toBeInTheDocument();
    });

    it("still shows a provider's refusal while password sign-in is turned off", () => {
        // given
        const siteInfo = { password_sign_in: false, identity_providers: [google] };

        // when
        renderWithProviders(<LoginPage />, { route: "/login?error=account_not_allowed&provider=google", siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("That Google account may not sign in to this site.");
    });

    it("starts the chosen provider's sign-in while password sign-in is turned off", async () => {
        // given
        const providerSignIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(providerSignIn);
        const siteInfo = { password_sign_in: false, identity_providers: [google] };
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(providerSignIn.start).toHaveBeenCalledWith(google, { purpose: "sign_in", return_to: undefined });
    });

    it("starts a sign-in with no return path, so the server picks where a new account lands", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledWith(google, { purpose: "sign_in", return_to: undefined });
    });

    it("starts the sign-in of the provider chosen when the site offers two", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo: { identity_providers: [google, example] } });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Example ID" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(example, { purpose: "sign_in", return_to: undefined });
    });

    it("starts a sign-in that returns to the page that asked the visitor to sign in", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderFromProtectedPage({ siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledWith(google, { purpose: "sign_in", return_to: "/account" });
    });

    it("clears the form's messages when a provider is chosen instead", async () => {
        // given
        const login = makeMutationState();
        useLogin.mockReturnValue(login);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo: { identity_providers: [google] } });
        await user.click(screen.getByRole("button", { name: "Sign in" }));
        login.reset.mockClear();

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(screen.queryByText("Enter your email address.")).not.toBeInTheDocument();
        expect(login.reset).toHaveBeenCalledOnce();
    });

    it("clears a provider's failure when the form is sent instead", async () => {
        // given
        const signIn = makeProviderSignIn({
            provider: google,
            error: makeApiError({ status: 503, message: "service unavailable" }),
        });
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login", siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(signIn.reset).toHaveBeenCalledOnce();
    });
});

describe("LoginPage after a provider refused the sign-in", () => {
    const siteInfo = { identity_providers: [google] };

    it.each(CALLBACK_CODES)("explains the %s refusal", code => {
        // given
        const route = `/login?error=${code}&provider=google`;

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(providerCallbackMessage(code, "sign_in", "Google") ?? "");
    });

    it("offers the owner of an unverified account a new link in place of signing in", () => {
        // given
        const route = "/login?error=account_unverified&provider=google";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo, user: null });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("Verify your email address before you sign in.");
        expect(alert).toHaveFocus();
        expect(screen.getByLabelText("Email address")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Send new link" })).toBeInTheDocument();
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Continue with Google" })).not.toBeInTheDocument();
    });

    it("sends a new link to the address the owner of an unverified account enters", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login?error=account_unverified&provider=google", siteInfo });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send new link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("goes back to the sign-in form from the unverified account's refusal", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login?error=account_unverified&provider=google", siteInfo });

        // when
        await user.click(screen.getByRole("link", { name: "Back to sign in" }));

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(screen.getByLabelText("Password")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
    });

    it("tells the owner of a matching password account how to link the provider", () => {
        // given
        const route = "/login?error=link_required&provider=google";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "An account already uses this email address. Sign in with your password, or reset it if you never set one, then link Google from your account settings.",
        );
    });

    it("names the provider the refusal came from when the site offers two", () => {
        // given
        const route = "/login?error=cancelled&provider=example";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo: { identity_providers: [google, example] } });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("You cancelled signing in with Example ID.");
    });

    it("never repeats the words of a provider the site does not list", () => {
        // given
        const route = `/login?error=cancelled&provider=${encodeURIComponent("Support: call 0800 000 000")}`;

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Signing in failed. Try again.");
        expect(screen.getByRole("alert")).not.toHaveTextContent("Support");
    });

    it("moves focus to the refusal", () => {
        // given
        const route = "/login?error=cancelled&provider=google";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("treats a refusal code it does not know as a failed sign-in with that provider", () => {
        // given
        const route = "/login?error=something_new&provider=google";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Signing in with Google failed. Try again.");
    });

    it("says only that signing in failed when the refusal names no provider", () => {
        // given
        const route = "/login?error=cancelled";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Signing in failed. Try again.");
    });

    it("shows nothing for an empty refusal code", () => {
        // given
        const route = "/login?error=&provider=google";

        // when
        renderWithProviders(<LoginPage />, { route, siteInfo });

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("sends a signed-in visitor to their account settings with a refusal from linking a provider", () => {
        // given
        const ui = (
            <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/account" element={<AccountProbe />} />
                <Route path="/" element={<p>the home page</p>} />
            </Routes>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser(), route: "/login?error=expired&provider=google", siteInfo });

        // then
        expect(screen.getByText("the account page?error=expired&provider=google")).toBeInTheDocument();
        expect(screen.queryByText("the home page")).not.toBeInTheDocument();
    });

    it("sends a signed-in visitor's refusal on without a provider when it named none", () => {
        // given
        const ui = (
            <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/account" element={<AccountProbe />} />
            </Routes>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser(), route: "/login?error=failed", siteInfo });

        // then
        expect(screen.getByText("the account page?error=failed")).toBeInTheDocument();
    });

    it("sends a visitor who signs in with a password after a provider's refusal on to the home page", async () => {
        // given
        const options: ProviderOptions = { route: "/login?error=cancelled&provider=google", siteInfo };
        const ui = (
            <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/account" element={<AccountProbe />} />
                <Route path="/" element={<p>the home page</p>} />
            </Routes>
        );
        const { rerender } = renderWithProviders(ui, options);
        const user = userEvent.setup();
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // when
        options.user = makeSessionUser();
        rerender(ui);

        // then
        expect(screen.getByText("the home page")).toBeInTheDocument();
        expect(screen.queryByText(/the account page/)).not.toBeInTheDocument();
    });

    it("stops showing the refusal once the visitor signs in with a password instead", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<LoginPage />, { route: "/login?error=expired&provider=google", siteInfo });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Sign in" }));

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
});
