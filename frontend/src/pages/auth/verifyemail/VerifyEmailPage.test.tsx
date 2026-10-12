import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { VerifyEmailPage } from "./VerifyEmailPage";

const { useVerifyEmail, useResendVerification, readLocationToken, clearLocationToken } = vi.hoisted(() => ({
    useVerifyEmail: vi.fn(),
    useResendVerification: vi.fn(),
    readLocationToken: vi.fn(),
    clearLocationToken: vi.fn(),
}));

vi.mock("../../../hooks/mutations/auth", () => ({ useVerifyEmail, useResendVerification }));
vi.mock("../../../platform/location", () => ({ readLocationToken, clearLocationToken }));

beforeEach(() => {
    useVerifyEmail.mockReturnValue(makeMutationState());
    useResendVerification.mockReturnValue(makeMutationState());
    readLocationToken.mockReset();
    readLocationToken.mockReturnValue("verify-token-1");
    clearLocationToken.mockReset();
});

describe("VerifyEmailPage", () => {
    it("verifies the address with the code from the link", () => {
        // given
        const verify = makeMutationState();
        useVerifyEmail.mockReturnValue(verify);

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        expect(verify.mutate).toHaveBeenCalledOnce();
        expect(verify.mutate).toHaveBeenCalledWith({ token: "verify-token-1" });
    });

    it("verifies only once when React runs the page's effects twice", () => {
        // given
        const verify = makeMutationState();
        useVerifyEmail.mockReturnValue(verify);

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" }, { reactStrictMode: true });

        // then
        expect(verify.mutate).toHaveBeenCalledOnce();
    });

    it("takes the code out of the address bar once it has read it", () => {
        // given
        const route = "/verify-email";

        // when
        renderWithProviders(<VerifyEmailPage />, { route });

        // then
        expect(clearLocationToken).toHaveBeenCalled();
    });

    it("says it is checking the link while the server answers", () => {
        // given
        useVerifyEmail.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Verify your email address" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Checking your link...");
    });

    it("says the address is verified and offers Sign in to a visitor who is signed out", () => {
        // given
        useVerifyEmail.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email", user: null });

        // then
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent("Your email address is verified.");
        expect(notice).toHaveFocus();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        expect(screen.queryByRole("link", { name: "Continue" })).not.toBeInTheDocument();
    });

    it("offers Sign in to a signed-in user too, because it assumes nothing about the session", () => {
        // given
        useVerifyEmail.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email", user: makeSessionUser() });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Your email address is verified.");
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });

    it("shows the server's reason when the link no longer works and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 400, message: "the verification link is invalid or has expired" });
        useVerifyEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("the verification link is invalid or has expired");
        expect(alert).toHaveFocus();
        expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
    });

    it("shows the server's reason when another account took the new address first", () => {
        // given
        const failure = makeApiError({ status: 409, message: "that email address is already in use" });
        useVerifyEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("that email address is already in use");
        expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
    });

    it("offers to try the same link again when the server failed", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to verify the email address" });
        const verify = makeMutationState({ isError: true, error: failure });
        useVerifyEmail.mockReturnValue(verify);
        const user = userEvent.setup();
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });
        verify.mutate.mockClear();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to verify the email address");
        expect(verify.mutate).toHaveBeenCalledWith({ token: "verify-token-1" });
    });

    it("says the link is missing its code when it has none, and sends nothing", () => {
        // given
        readLocationToken.mockReturnValue("");
        const verify = makeMutationState();
        useVerifyEmail.mockReturnValue(verify);

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("This link is missing its code. Open the link from your email again.");
        expect(alert).toHaveFocus();
        expect(verify.mutate).not.toHaveBeenCalled();
    });
});

describe("VerifyEmailPage offering a new link", () => {
    beforeEach(() => {
        readLocationToken.mockReturnValue("");
    });

    it("asks a signed-out visitor for their address instead of asking them to sign in", () => {
        // given
        const user = null;

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email", user });

        // then
        expect(screen.getByLabelText("Email address")).toHaveAttribute("type", "email");
        expect(screen.getByRole("button", { name: "Send new link" })).toBeInTheDocument();
        expect(screen.queryByText("to get a new link.", { exact: false })).not.toBeInTheDocument();
    });

    it("sends a new link to the address entered, with no session", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email", user: null });

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send new link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("offers a new link after a link that no longer works too", () => {
        // given
        readLocationToken.mockReturnValue("verify-token-1");
        const failure = makeApiError({ status: 400, message: "the verification link is invalid or has expired" });
        useVerifyEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email", user: null });

        // then
        expect(screen.getByRole("button", { name: "Send new link" })).toBeInTheDocument();
    });

    it("offers a new link beside trying the same link again when the server failed", () => {
        // given
        readLocationToken.mockReturnValue("verify-token-1");
        const failure = makeApiError({ status: 500, message: "failed to verify the email address" });
        useVerifyEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Send new link" })).toBeInTheDocument();
    });

    it("offers the new link without waiting for the sign-in check, because it needs no session", () => {
        // given
        const auth = { loading: true };

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email", auth });

        // then
        expect(screen.getByRole("button", { name: "Send new link" })).toBeInTheDocument();
    });

    it("shows the server's reason when a new link could not be sent", () => {
        // given
        const failure = makeApiError({ status: 503, message: "email is not available on this site" });
        useResendVerification.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<VerifyEmailPage />, { route: "/verify-email" });

        // then
        expect(screen.getByText("email is not available on this site")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Send new link" })).toBeEnabled();
    });

    it("links back to sign in", () => {
        // given
        const route = "/verify-email";

        // when
        renderWithProviders(<VerifyEmailPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Back to sign in" })).toHaveAttribute("href", "/login");
    });
});
