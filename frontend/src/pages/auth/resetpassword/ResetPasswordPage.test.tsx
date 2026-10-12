import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { ResetPasswordPage } from "./ResetPasswordPage";

const { useResetPassword, readLocationToken, clearLocationToken } = vi.hoisted(() => ({
    useResetPassword: vi.fn(),
    readLocationToken: vi.fn(),
    clearLocationToken: vi.fn(),
}));

vi.mock("../../../hooks/mutations/auth", () => ({ useResetPassword }));
vi.mock("../../../platform/location", () => ({ readLocationToken, clearLocationToken }));

async function choose(user: ReturnType<typeof userEvent.setup>, password: string, confirmation: string) {
    await user.type(screen.getByLabelText("New password"), password);
    await user.type(screen.getByLabelText("Confirm new password"), confirmation);
    await user.click(screen.getByRole("button", { name: "Reset password" }));
}

beforeEach(() => {
    useResetPassword.mockReturnValue(makeMutationState());
    readLocationToken.mockReset();
    readLocationToken.mockReturnValue("reset-token-1");
    clearLocationToken.mockReset();
});

describe("ResetPasswordPage", () => {
    it("asks for the new password twice", () => {
        // given
        const route = "/reset-password";

        // when
        renderWithProviders(<ResetPasswordPage />, { route });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Choose a new password" })).toBeInTheDocument();
        expect(screen.getByLabelText("New password")).toHaveAttribute("type", "password");
        expect(screen.getByLabelText("Confirm new password")).toHaveAttribute("type", "password");
        expect(screen.getByRole("button", { name: "Reset password" })).toBeInTheDocument();
    });

    it("resets the password with the code from the link", async () => {
        // given
        const reset = makeMutationState();
        useResetPassword.mockReturnValue(reset);
        const user = userEvent.setup();
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // when
        await choose(user, "harbour-lights", "harbour-lights");

        // then
        expect(reset.mutate).toHaveBeenCalledOnce();
        expect(reset.mutate).toHaveBeenCalledWith({ token: "reset-token-1", new_password: "harbour-lights" });
    });

    it("takes the code out of the address bar once it has read it", () => {
        // given
        const route = "/reset-password";

        // when
        renderWithProviders(<ResetPasswordPage />, { route });

        // then
        expect(clearLocationToken).toHaveBeenCalled();
    });

    it("says what is missing, ties each message to its field and moves focus to the summary", async () => {
        // given
        const reset = makeMutationState();
        useResetPassword.mockReturnValue(reset);
        const user = userEvent.setup();
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // when
        await user.click(screen.getByRole("button", { name: "Reset password" }));

        // then
        expect(screen.getByLabelText("New password")).toHaveAccessibleDescription("Enter a new password.");
        expect(screen.getByLabelText("Confirm new password")).toHaveAccessibleDescription(
            "Enter the new password again.",
        );
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(reset.mutate).not.toHaveBeenCalled();
        expect(reset.reset).toHaveBeenCalled();
    });

    it("refuses two passwords that do not match", async () => {
        // given
        const reset = makeMutationState();
        useResetPassword.mockReturnValue(reset);
        const user = userEvent.setup();
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // when
        await choose(user, "harbour-lights", "harbour-light");

        // then
        expect(screen.getByLabelText("Confirm new password")).toHaveAccessibleDescription(
            "The passwords do not match.",
        );
        expect(screen.getByLabelText("New password")).not.toBeInvalid();
        expect(screen.getByRole("alert")).toHaveTextContent("The passwords do not match.");
        expect(reset.mutate).not.toHaveBeenCalled();
    });

    it("ties a password the server finds too short to the new password field", () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "the password is too short",
            code: "password_too_short",
        });
        useResetPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // then
        expect(screen.getByLabelText("New password")).toHaveAccessibleDescription("Choose a longer password.");
        expect(screen.getByRole("alert")).toHaveTextContent("Choose a longer password.");
    });

    it("shows the server's reason when the link no longer works, and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 400, message: "the reset link is invalid or has expired" });
        useResetPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("the reset link is invalid or has expired");
        expect(alert).toHaveFocus();
    });

    it("shows the server's reason when the password is too long", () => {
        // given
        const failure = makeApiError({ status: 400, message: "the password is longer than 72 bytes" });
        useResetPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the password is longer than 72 bytes");
    });

    it("holds the button while the password is being reset", () => {
        // given
        useResetPassword.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // then
        expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();
    });

    it("offers a new link from the form, for a link that has expired", () => {
        // given
        const route = "/reset-password";

        // when
        renderWithProviders(<ResetPasswordPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Get a new reset link" })).toHaveAttribute("href", "/forgot-password");
    });

    it("says the password is reset and that every device is signed out, then offers sign in", () => {
        // given
        useResetPassword.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password" });

        // then
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent("Your password has been reset, and you are signed out on every device.");
        expect(notice).toHaveFocus();
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        expect(screen.queryByLabelText("New password")).not.toBeInTheDocument();
    });
});

describe("ResetPasswordPage without a code in the link", () => {
    beforeEach(() => {
        readLocationToken.mockReturnValue("");
    });

    it("says the link is missing its code and draws no form", () => {
        // given
        const route = "/reset-password";

        // when
        renderWithProviders(<ResetPasswordPage />, { route });

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("This link is missing its code. Open the link from your email again.");
        expect(alert).toHaveFocus();
        expect(screen.queryByLabelText("New password")).not.toBeInTheDocument();
    });

    it("offers a new reset link", () => {
        // given
        const route = "/reset-password";

        // when
        renderWithProviders(<ResetPasswordPage />, { route });

        // then
        expect(screen.getByRole("link", { name: "Get a new reset link" })).toHaveAttribute("href", "/forgot-password");
    });
});

describe("ResetPasswordPage while password sign-in is off", () => {
    it("says password sign-in is off, draws no form and links back to sign in", () => {
        // given
        const siteInfo = { password_sign_in: false };

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password", siteInfo });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Choose a new password" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Password sign-in is turned off on this site.");
        expect(screen.queryByLabelText("New password")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Reset password" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to sign in" })).toHaveAttribute("href", "/login");
    });

    it("still takes the code out of the address bar", () => {
        // given
        const siteInfo = { password_sign_in: false };

        // when
        renderWithProviders(<ResetPasswordPage />, { route: "/reset-password", siteInfo });

        // then
        expect(clearLocationToken).toHaveBeenCalled();
    });
});
