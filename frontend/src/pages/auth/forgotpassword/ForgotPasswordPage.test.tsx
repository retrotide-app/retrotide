import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeTurnstileControl } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { ForgotPasswordPage } from "./ForgotPasswordPage";

const { useForgotPassword, useTurnstile } = vi.hoisted(() => ({
    useForgotPassword: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../../hooks/mutations/auth", () => ({ useForgotPassword }));
vi.mock("../../../hooks/useTurnstile", () => ({ useTurnstile }));

function settlingMutation() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

beforeEach(() => {
    useForgotPassword.mockReturnValue(makeMutationState());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("ForgotPasswordPage", () => {
    it("asks for the email address the account signs in with", () => {
        // given
        const route = "/forgot-password";

        // when
        renderWithProviders(<ForgotPasswordPage />, { route });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Reset your password" })).toBeInTheDocument();
        expect(screen.getByLabelText("Email address")).toHaveAttribute("type", "email");
        expect(screen.getByRole("button", { name: "Send reset link" })).toBeInTheDocument();
    });

    it("asks for a reset link for the address entered", async () => {
        // given
        const forgot = makeMutationState();
        useForgotPassword.mockReturnValue(forgot);
        const user = userEvent.setup();
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // when
        await user.type(screen.getByLabelText("Email address"), " sam@example.test ");
        await user.click(screen.getByRole("button", { name: "Send reset link" }));

        // then
        expect(forgot.mutate).toHaveBeenCalledOnce();
        expect(forgot.mutate).toHaveBeenCalledWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("asks for an address before sending anything, and moves focus to the summary", async () => {
        // given
        const forgot = makeMutationState();
        useForgotPassword.mockReturnValue(forgot);
        const user = userEvent.setup();
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // when
        await user.click(screen.getByRole("button", { name: "Send reset link" }));

        // then
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription("Enter your email address.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(forgot.mutate).not.toHaveBeenCalled();
        expect(forgot.reset).toHaveBeenCalled();
    });

    it("gives the same answer whether or not the address has an account", async () => {
        // given
        const forgot = makeMutationState();
        useForgotPassword.mockReturnValue(forgot);
        const user = userEvent.setup();
        const { rerender } = renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send reset link" }));

        // when
        useForgotPassword.mockReturnValue(makeMutationState({ isSuccess: true }));
        rerender(<ForgotPasswordPage />);

        // then
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent(
            "Check your inbox. If an account uses sam@example.test, we sent it a link to reset the password. The link expires after one hour.",
        );
        expect(notice).toHaveFocus();
        expect(screen.queryByRole("button", { name: "Send reset link" })).not.toBeInTheDocument();
    });

    it("says plainly that nothing was sent when the site cannot send email, and moves focus to it", () => {
        // given
        const failure = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });
        useForgotPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("This site cannot send email yet, so nothing was sent.");
        expect(alert).toHaveFocus();
        expect(screen.queryByText(/Check your inbox/)).not.toBeInTheDocument();
    });

    it("shows the server's reason when too many links were asked for", () => {
        // given
        const failure = makeApiError({ status: 429, message: "too many requests, please try again later" });
        useForgotPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("too many requests, please try again later");
    });

    it("holds the button while the link is being sent", () => {
        // given
        useForgotPassword.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // then
        expect(screen.getByRole("button", { name: "Send reset link" })).toBeDisabled();
    });

    it("links back to sign in", () => {
        // given
        const route = "/forgot-password";

        // when
        renderWithProviders(<ForgotPasswordPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Back to sign in" })).toHaveAttribute("href", "/login");
    });
});

describe("ForgotPasswordPage with Turnstile", () => {
    it("draws the verification check while the site has it switched on", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: true });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // then
        expect(turnstile.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("asks for the verification check before sending anything", async () => {
        // given
        const forgot = makeMutationState();
        useForgotPassword.mockReturnValue(forgot);
        useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
        const user = userEvent.setup();
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send reset link" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Complete the verification check.");
        expect(forgot.mutate).not.toHaveBeenCalled();
    });

    it("sends the verification token with the request, then resets the check for the next try", async () => {
        // given
        const forgot = settlingMutation();
        useForgotPassword.mockReturnValue(forgot);
        const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-1" });
        useTurnstile.mockReturnValue(turnstile);
        const user = userEvent.setup();
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password" });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send reset link" }));

        // then
        expect(forgot.mutate).toHaveBeenCalledWith(
            { email: "sam@example.test", turnstile_token: "turnstile-1" },
            expect.anything(),
        );
        expect(turnstile.reset).toHaveBeenCalledOnce();
    });
});

describe("ForgotPasswordPage while password sign-in is off", () => {
    it("says password sign-in is off, draws no form and links back to sign in", () => {
        // given
        const siteInfo = { password_sign_in: false };

        // when
        renderWithProviders(<ForgotPasswordPage />, { route: "/forgot-password", siteInfo });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Reset your password" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Password sign-in is turned off on this site.");
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Send reset link" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to sign in" })).toHaveAttribute("href", "/login");
    });
});
