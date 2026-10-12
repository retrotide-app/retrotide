import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeIdentityProvider,
    makeMutationState,
    makeProviderSignIn,
    makeSessionUser,
    makeTurnstileControl,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { RegisterPage } from "./RegisterPage";

const { useRegister, useResendVerification, useProviderSignIn, useTurnstile } = vi.hoisted(() => ({
    useRegister: vi.fn(),
    useResendVerification: vi.fn(),
    useProviderSignIn: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../../hooks/mutations/auth", () => ({ useRegister, useResendVerification }));
vi.mock("../../../hooks/useProviderSignIn", () => ({ useProviderSignIn }));
vi.mock("../../../hooks/useTurnstile", () => ({ useTurnstile }));

const google = makeIdentityProvider();
const example = makeIdentityProvider({ id: "example", name: "Example ID" });
const privateWithAccounts = { site_mode: "private" as const, registration_open: false };
const privateAndEmpty = { site_mode: "private" as const, registration_open: true };
const SITE_PRIVATE = "This site is private. Ask an admin to invite you.";
const REGISTRATION_CLOSED = "New accounts are closed for now. If you already have an account, sign in.";
const publicAndClosed = { site_mode: "public" as const, registration_open: false };
const PASSWORD_SIGN_UP_WAITS_FOR_EMAIL =
    "Signing up with an email address and a password is closed until this site can send email.";

function settlingMutation() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

async function fillIn(user: ReturnType<typeof userEvent.setup>) {
    await user.type(screen.getByLabelText("Name (optional)"), " Sam ");
    await user.type(screen.getByLabelText("Email address"), "sam@example.test");
    await user.type(screen.getByLabelText("Password"), "harbour-lights");
}

function registered(verificationRequired: boolean) {
    return makeMutationState({
        isSuccess: true,
        data: { email: "sam@example.test", verification_required: verificationRequired },
    });
}

beforeEach(() => {
    useRegister.mockReturnValue(makeMutationState());
    useResendVerification.mockReturnValue(makeMutationState());
    useProviderSignIn.mockReturnValue(makeProviderSignIn());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("RegisterPage", () => {
    it("asks for a name, an email address and a password", () => {
        // given
        const route = "/register";

        // when
        renderWithProviders(<RegisterPage />, { route });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Create an account" })).toBeInTheDocument();
        expect(screen.getByLabelText("Name (optional)")).toBeInTheDocument();
        expect(screen.getByLabelText("Email address")).toHaveAttribute("type", "email");
        expect(screen.getByLabelText("Password")).toHaveAttribute("type", "password");
        expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
    });

    it("stops the name at the length the server keeps, and says so", () => {
        // given
        const route = "/register";

        // when
        renderWithProviders(<RegisterPage />, { route });

        // then
        expect(screen.getByLabelText("Name (optional)")).toHaveAttribute("maxlength", "40");
        expect(screen.getByLabelText("Name (optional)")).toHaveAccessibleDescription("Up to 40 characters.");
    });

    it("creates the account with the details entered", async () => {
        // given
        const register = makeMutationState();
        useRegister.mockReturnValue(register);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(register.mutate).toHaveBeenCalledOnce();
        expect(register.mutate).toHaveBeenCalledWith(
            {
                email: "sam@example.test",
                password: "harbour-lights",
                display_name: "Sam",
                turnstile_token: undefined,
            },
            expect.anything(),
        );
    });

    it("says nothing about a private site on a public one", () => {
        // given
        const siteInfo = { site_mode: "public" as const, registration_open: true };

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.queryByText(SITE_PRIVATE)).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
    });

    it("says what is missing, ties each message to its field and moves focus to the summary", async () => {
        // given
        const register = makeMutationState();
        useRegister.mockReturnValue(register);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // when
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription("Enter your email address.");
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Enter a password.");
        expect(screen.getByLabelText("Name (optional)")).not.toBeInvalid();
        const summary = screen.getByRole("alert");
        expect(summary).toHaveTextContent("Enter your email address. Enter a password.");
        expect(summary).toHaveFocus();
        expect(register.mutate).not.toHaveBeenCalled();
        expect(register.reset).toHaveBeenCalled();
    });

    it("says the site is private when the server refuses the registration for that reason", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "this site is private, ask an admin to invite you",
            code: "site_private",
        });
        useRegister.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent(SITE_PRIVATE);
        expect(alert).toHaveFocus();
    });

    it("ties a password the server finds too short to the password field", () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "the password is too short",
            code: "password_too_short",
        });
        useRegister.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Choose a longer password.");
        expect(screen.getByRole("alert")).toHaveTextContent("Choose a longer password.");
    });

    it("ties an address that already has an account to the email field", () => {
        // given
        const failure = makeApiError({ status: 409, message: "that email address is already in use" });
        useRegister.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription(
            "An account already uses this email address.",
        );
        expect(screen.getByRole("alert")).toHaveTextContent("An account already uses this email address.");
    });

    it("shows the server's reason for an address it cannot use", () => {
        // given
        const failure = makeApiError({ status: 400, message: "a valid email address is required" });
        useRegister.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("a valid email address is required");
    });

    it("holds the button while the account is being created", () => {
        // given
        useRegister.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        expect(screen.getByRole("button", { name: "Create account" })).toBeDisabled();
    });

    it("sends a signed-in visitor to the home page", () => {
        // given
        const ui = (
            <Routes>
                <Route path="/register" element={<RegisterPage />} />
                <Route path="/" element={<p>the home page</p>} />
            </Routes>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser(), route: "/register" });

        // then
        expect(screen.getByText("the home page")).toBeInTheDocument();
    });

    it("links to sign in for someone who already has an account", () => {
        // given
        const route = "/register";

        // when
        renderWithProviders(<RegisterPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });
});

describe("RegisterPage once the account is created", () => {
    it("asks the new account to check its inbox, naming the address, instead of signing it in", () => {
        // given
        useRegister.mockReturnValue(registered(true));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", user: null });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Check your inbox" })).toBeInTheDocument();
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent(
            "We sent a link to sam@example.test. Open it to verify your email address, then sign in.",
        );
        expect(notice).toHaveFocus();
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
    });

    it("links to sign in for when the address is verified", () => {
        // given
        useRegister.mockReturnValue(registered(true));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("sends the link again to the address the account was created with", async () => {
        // given
        useRegister.mockReturnValue(registered(true));
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // when
        await user.click(screen.getByRole("button", { name: "Resend link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("says the link was sent again", () => {
        // given
        useRegister.mockReturnValue(registered(true));
        useResendVerification.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        const notices = screen.getAllByRole("status").map(notice => notice.textContent);
        expect(notices).toContain("Link sent to sam@example.test. Check your inbox.");
        expect(screen.queryByRole("button", { name: "Resend link" })).not.toBeInTheDocument();
    });

    it("tells the first account, which starts verified, that it is ready and to sign in", () => {
        // given
        useRegister.mockReturnValue(registered(false));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", user: null });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Your account is ready" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Sign in to get started.");
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        expect(screen.queryByRole("button", { name: "Resend link" })).not.toBeInTheDocument();
        expect(screen.queryByText(/Check your inbox/)).not.toBeInTheDocument();
    });
});

describe("RegisterPage on a private site that already has an account", () => {
    it("says the site is private in place of the form", () => {
        // given
        const siteInfo = privateWithAccounts;

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Create an account" })).toBeInTheDocument();
        expect(screen.getByText(SITE_PRIVATE)).toBeInTheDocument();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("links to sign in", () => {
        // given
        const siteInfo = privateWithAccounts;

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("still sends a signed-in visitor to the home page", () => {
        // given
        const ui = (
            <Routes>
                <Route path="/register" element={<RegisterPage />} />
                <Route path="/" element={<p>the home page</p>} />
            </Routes>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser(), route: "/register", siteInfo: privateWithAccounts });

        // then
        expect(screen.getByText("the home page")).toBeInTheDocument();
    });
});

describe("RegisterPage on a public site that has closed new accounts", () => {
    it("says new accounts are closed in place of the form, without calling the site private", () => {
        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: publicAndClosed });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Create an account" })).toBeInTheDocument();
        expect(screen.getByText(REGISTRATION_CLOSED)).toBeInTheDocument();
        expect(screen.queryByText(SITE_PRIVATE)).not.toBeInTheDocument();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("says new accounts are closed when the server refuses the registration for that reason", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "sign-up is closed until this site can send email",
            code: "registration_closed",
        });
        useRegister.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent(REGISTRATION_CLOSED);
        expect(alert).toHaveFocus();
    });
});

describe("RegisterPage on a private site with no account yet", () => {
    it("lets the first account register, because the server always admits it", async () => {
        // given
        const register = makeMutationState();
        useRegister.mockReturnValue(register);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: privateAndEmpty });

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(screen.queryByText(SITE_PRIVATE)).not.toBeInTheDocument();
        expect(register.mutate).toHaveBeenCalledExactlyOnceWith(
            {
                email: "sam@example.test",
                password: "harbour-lights",
                display_name: "Sam",
                turnstile_token: undefined,
            },
            expect.anything(),
        );
    });

    it("says the site is private when another account got there first", () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "this site is private, ask an admin to invite you",
            code: "site_private",
        });
        useRegister.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: privateAndEmpty });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(SITE_PRIVATE);
    });

    it("tells the first account it is ready once it is created, even though the site now counts as private", () => {
        // given
        useRegister.mockReturnValue(registered(false));

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: privateWithAccounts });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Your account is ready" })).toBeInTheDocument();
        expect(screen.queryByText(SITE_PRIVATE)).not.toBeInTheDocument();
    });
});

describe("RegisterPage with Turnstile", () => {
    it("draws the verification check while the site has it switched on", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: true });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // then
        expect(turnstile.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("asks for the verification check before sending anything", async () => {
        // given
        const register = makeMutationState();
        useRegister.mockReturnValue(register);
        useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Complete the verification check.");
        expect(register.mutate).not.toHaveBeenCalled();
    });

    it("sends the verification token with the registration, then resets the check for the next try", async () => {
        // given
        const register = settlingMutation();
        useRegister.mockReturnValue(register);
        const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-1" });
        useTurnstile.mockReturnValue(turnstile);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register" });

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(register.mutate).toHaveBeenCalledWith(
            expect.objectContaining({ turnstile_token: "turnstile-1" }),
            expect.anything(),
        );
        expect(turnstile.reset).toHaveBeenCalledOnce();
    });
});

describe("RegisterPage with sign-in providers", () => {
    it("offers no provider sign-up while the site has none switched on", () => {
        // given
        const siteInfo = { identity_providers: [] };

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.queryByRole("button", { name: /^Continue with/ })).not.toBeInTheDocument();
    });

    it("offers one button for each provider the site has switched on", () => {
        // given
        const siteInfo = { identity_providers: [google, example] };

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Example ID" })).toBeInTheDocument();
    });

    it("starts a provider sign-up", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(google, { purpose: "sign_in", return_to: undefined });
    });

    it("starts the sign-up of the provider chosen when the site offers two", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, {
            route: "/register",
            siteInfo: { identity_providers: [google, example] },
        });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Example ID" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(example, { purpose: "sign_in", return_to: undefined });
    });

    it("clears the form's failure when a provider is chosen instead", async () => {
        // given
        const register = makeMutationState();
        useRegister.mockReturnValue(register);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
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
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: { identity_providers: [google] } });

        // when
        await user.click(screen.getByRole("button", { name: "Create account" }));

        // then
        expect(signIn.reset).toHaveBeenCalledOnce();
    });
});

describe("RegisterPage while password sign-in is off", () => {
    const providersOnly = { password_sign_in: false, identity_providers: [google, example] };

    it("offers only the providers, with no form and no divider", () => {
        // given
        const siteInfo = providersOnly;

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Create an account" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Example ID" })).toBeInTheDocument();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
        expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
        expect(screen.queryByText("or")).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("starts a provider sign-up", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = userEvent.setup();
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo: providersOnly });

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Example ID" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(example, { purpose: "sign_in", return_to: undefined });
    });

    it("still offers the providers when the site cannot send email, since they need none", () => {
        // given
        const siteInfo = { ...providersOnly, ...publicAndClosed };

        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.queryByText(REGISTRATION_CLOSED)).not.toBeInTheDocument();
        expect(screen.queryByText(PASSWORD_SIGN_UP_WAITS_FOR_EMAIL)).not.toBeInTheDocument();
    });
});

describe("RegisterPage on a public site that cannot send email but offers providers", () => {
    const siteInfo = { ...publicAndClosed, identity_providers: [google] };

    it("offers the providers and says only email and password sign-up waits for email", () => {
        // when
        renderWithProviders(<RegisterPage />, { route: "/register", siteInfo });

        // then
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
        expect(screen.getByText(PASSWORD_SIGN_UP_WAITS_FOR_EMAIL)).toBeInTheDocument();
        expect(screen.queryByText(REGISTRATION_CLOSED)).not.toBeInTheDocument();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Create account" })).not.toBeInTheDocument();
    });
});
