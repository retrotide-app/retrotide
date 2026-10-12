import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser, makeTurnstileControl } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { PasswordSection } from "./PasswordSection";

const { useChangePassword, useForgotPassword, useTurnstile } = vi.hoisted(() => ({
    useChangePassword: vi.fn(),
    useForgotPassword: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../hooks/mutations/auth", () => ({ useChangePassword, useForgotPassword }));
vi.mock("../../hooks/useTurnstile", () => ({ useTurnstile }));

const withPassword = makeSessionUser({ has_password: true });
const withoutPassword = makeSessionUser({ email: "sam@gmail.com", has_password: false });

function succeeding() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
            options?.onSuccess?.();
        }),
    });
}

function settling() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

function renderWithPassword(onPasswordChanged = vi.fn()) {
    renderWithProviders(<PasswordSection user={withPassword} onPasswordChanged={onPasswordChanged} />, {
        user: withPassword,
    });

    return onPasswordChanged;
}

async function openDialog() {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Change password" }));

    return user;
}

function dialog() {
    return within(screen.getByRole("dialog", { name: "Change password" }));
}

beforeEach(() => {
    useChangePassword.mockReturnValue(makeMutationState());
    useForgotPassword.mockReturnValue(makeMutationState());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("PasswordSection", () => {
    describe("for an account with a password", () => {
        it("keeps the fields in a dialog until asked", () => {
            // when
            renderWithPassword();

            // then
            expect(screen.getByRole("button", { name: "Change password" })).toBeInTheDocument();
            expect(screen.queryByLabelText("Current password")).not.toBeInTheDocument();
            expect(screen.queryByRole("button", { name: "Set a password" })).not.toBeInTheDocument();
        });

        it("asks for the current password and the new one, the first focused", async () => {
            // given
            renderWithPassword();

            // when
            await openDialog();

            // then
            expect(dialog().getByLabelText("Current password")).toHaveAttribute("type", "password");
            expect(dialog().getByLabelText("Current password")).toHaveFocus();
            expect(dialog().getByLabelText("New password")).toHaveAttribute("autocomplete", "new-password");
        });

        it("changes the password", async () => {
            // given
            const change = makeMutationState();
            useChangePassword.mockReturnValue(change);
            renderWithPassword();
            const user = await openDialog();

            // when
            await user.type(dialog().getByLabelText("Current password"), "harbour-lights");
            await user.type(dialog().getByLabelText("New password"), "lantern-tide-42");
            await user.click(dialog().getByRole("button", { name: "Save new password" }));

            // then
            expect(change.mutate).toHaveBeenCalledWith(
                { current_password: "harbour-lights", new_password: "lantern-tide-42" },
                expect.objectContaining({ onSuccess: expect.any(Function) }),
            );
        });

        it("closes the dialog and reports the change once it is saved", async () => {
            // given
            useChangePassword.mockReturnValue(succeeding());
            const onPasswordChanged = renderWithPassword();
            const user = await openDialog();

            // when
            await user.type(dialog().getByLabelText("Current password"), "harbour-lights");
            await user.type(dialog().getByLabelText("New password"), "lantern-tide-42{Enter}");

            // then
            expect(onPasswordChanged).toHaveBeenCalledOnce();
            expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        });

        it("says the password changed and the other devices are signed out", () => {
            // given
            useChangePassword.mockReturnValue(makeMutationState({ isSuccess: true }));

            // when
            renderWithPassword();

            // then
            expect(screen.getByRole("status")).toHaveTextContent(
                "Password changed. You are signed out on every other device.",
            );
        });

        it("starts empty again after a cancel, without sending anything", async () => {
            // given
            const change = makeMutationState();
            useChangePassword.mockReturnValue(change);
            renderWithPassword();
            const user = await openDialog();
            await user.type(dialog().getByLabelText("Current password"), "harbour-lights");

            // when
            await user.click(dialog().getByRole("button", { name: "Cancel" }));
            await user.click(screen.getByRole("button", { name: "Change password" }));

            // then
            expect(dialog().getByLabelText("Current password")).toHaveValue("");
            expect(change.mutate).not.toHaveBeenCalled();
        });

        it("says what is missing before sending anything, ties each message to its field and moves focus to it", async () => {
            // given
            const change = makeMutationState();
            useChangePassword.mockReturnValue(change);
            renderWithPassword();
            const user = await openDialog();

            // when
            await user.click(dialog().getByRole("button", { name: "Save new password" }));

            // then
            expect(dialog().getByLabelText("Current password")).toHaveAccessibleDescription(
                "Enter your current password.",
            );
            expect(dialog().getByLabelText("New password")).toHaveAccessibleDescription("Enter a new password.");
            expect(dialog().getByRole("alert")).toHaveTextContent("Enter your current password. Enter a new password.");
            expect(dialog().getByRole("alert")).toHaveFocus();
            expect(change.mutate).not.toHaveBeenCalled();
            expect(change.reset).toHaveBeenCalled();
        });

        it("holds the button while the password is being changed", async () => {
            // given
            useChangePassword.mockReturnValue(makeMutationState({ isPending: true }));
            renderWithPassword();

            // when
            await openDialog();

            // then
            expect(dialog().getByRole("button", { name: "Save new password" })).toBeDisabled();
        });

        it("ties a wrong current password to its field", async () => {
            // given
            const failure = makeApiError({
                status: 403,
                message: "the current password is incorrect",
                code: "incorrect_password",
            });
            useChangePassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));
            renderWithPassword();

            // when
            await openDialog();

            // then
            expect(dialog().getByRole("alert")).toHaveTextContent("the current password is incorrect");
            expect(dialog().getByLabelText("Current password")).toHaveAccessibleDescription(
                "the current password is incorrect",
            );
            expect(dialog().getByLabelText("New password")).not.toHaveAttribute("aria-invalid");
        });

        it("ties a new password that is too short to its field", async () => {
            // given
            const failure = makeApiError({
                status: 400,
                message: "the password is too short",
                code: "password_too_short",
            });
            useChangePassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));
            renderWithPassword();

            // when
            await openDialog();

            // then
            expect(dialog().getByLabelText("New password")).toHaveAccessibleDescription("the password is too short");
            expect(dialog().getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
        });

        it("shows any other failure without blaming a field", async () => {
            // given
            const failure = makeApiError({ status: 400, message: "the password is longer than 72 bytes" });
            useChangePassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));
            renderWithPassword();

            // when
            await openDialog();

            // then
            expect(dialog().getByRole("alert")).toHaveTextContent("the password is longer than 72 bytes");
            expect(dialog().getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
            expect(dialog().getByLabelText("New password")).not.toHaveAttribute("aria-invalid");
        });
    });

    describe("for an account with no password", () => {
        it("offers to email a link that sets one", () => {
            // given
            const user = withoutPassword;

            // when
            renderWithProviders(<PasswordSection user={user} onPasswordChanged={vi.fn()} />, { user });

            // then
            expect(screen.getByText(/Your account has no password\. We will email a link to/)).toHaveTextContent(
                "Your account has no password. We will email a link to sam@gmail.com that lets you set one.",
            );
            expect(screen.getByRole("button", { name: "Set a password" })).toBeInTheDocument();
            expect(screen.queryByLabelText("Current password")).not.toBeInTheDocument();
        });

        it("sends the reset link to the account's own address", async () => {
            // given
            const request = makeMutationState();
            useForgotPassword.mockReturnValue(request);
            const user = userEvent.setup();
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // when
            await user.click(screen.getByRole("button", { name: "Set a password" }));

            // then
            expect(request.mutate).toHaveBeenCalledWith({ email: "sam@gmail.com" }, expect.anything());
        });

        it("asks for the verification check before sending anything", async () => {
            // given
            const request = makeMutationState();
            useForgotPassword.mockReturnValue(request);
            useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
            const user = userEvent.setup();
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // when
            await user.click(screen.getByRole("button", { name: "Set a password" }));

            // then
            expect(screen.getByRole("alert")).toHaveTextContent("Complete the verification check.");
            expect(screen.getByRole("alert")).toHaveFocus();
            expect(request.mutate).not.toHaveBeenCalled();
        });

        it("sends the Turnstile token when the site asks for one, then spends it", async () => {
            // given
            const request = settling();
            useForgotPassword.mockReturnValue(request);
            const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-token-1" });
            useTurnstile.mockReturnValue(turnstile);
            const user = userEvent.setup();
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // when
            await user.click(screen.getByRole("button", { name: "Set a password" }));

            // then
            expect(request.mutate).toHaveBeenCalledWith(
                { email: "sam@gmail.com", turnstile_token: "turnstile-token-1" },
                expect.anything(),
            );
            expect(turnstile.reset).toHaveBeenCalledOnce();
        });

        it("shows why the Turnstile check cannot run", () => {
            // given
            useTurnstile.mockReturnValue(
                makeTurnstileControl({ enabled: true, error: "The verification check failed. Try it again." }),
            );

            // when
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // then
            expect(screen.getByRole("alert")).toHaveTextContent("The verification check failed. Try it again.");
        });

        it("says where the link went", () => {
            // given
            useForgotPassword.mockReturnValue(makeMutationState({ isSuccess: true }));

            // when
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // then
            expect(screen.getByRole("status")).toHaveTextContent(
                "Check sam@gmail.com for the link. It works for one hour, and using it signs you out on every device.",
            );
        });

        it("holds the button while the link is being sent", () => {
            // given
            useForgotPassword.mockReturnValue(makeMutationState({ isPending: true }));

            // when
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // then
            expect(screen.getByRole("button", { name: "Set a password" })).toBeDisabled();
        });

        it("shows the server's reason when the link could not be sent and moves focus to it", () => {
            // given
            const failure = makeApiError({ status: 503, message: "email is not available on this site" });
            useForgotPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

            // when
            renderWithProviders(<PasswordSection user={withoutPassword} onPasswordChanged={vi.fn()} />, {
                user: withoutPassword,
            });

            // then
            expect(screen.getByRole("alert")).toHaveTextContent("email is not available on this site");
            expect(screen.getByRole("alert")).toHaveFocus();
        });
    });
});
