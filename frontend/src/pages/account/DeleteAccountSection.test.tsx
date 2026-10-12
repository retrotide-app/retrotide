import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionUser } from "../../types/app";
import { makeApiError, makeMutationState, makeSessionUser } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { DeleteAccountSection } from "./DeleteAccountSection";

const { useDeleteAccount } = vi.hoisted(() => ({ useDeleteAccount: vi.fn() }));

vi.mock("../../hooks/mutations/auth", () => ({ useDeleteAccount }));

const account = makeSessionUser({ has_password: true });

function renderSection(user: SessionUser = account) {
    return renderWithProviders(
        <Routes>
            <Route path="/account" element={<DeleteAccountSection user={user} />} />
            <Route path="/" element={<p>the home page</p>} />
        </Routes>,
        { user, route: "/account" },
    );
}

async function openDialog() {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Delete account" }));

    return user;
}

beforeEach(() => {
    useDeleteAccount.mockReturnValue(makeMutationState());
});

describe("DeleteAccountSection", () => {
    it("says the deletion cannot be undone", () => {
        // given
        const user = account;

        // when
        renderSection(user);

        // then
        expect(screen.getByRole("region", { name: "Delete account" })).toHaveTextContent(
            "This deletes your account and signs you out. You cannot undo it.",
        );
    });

    it("asks for the password before it deletes anything", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteAccount.mockReturnValue(deletion);
        renderSection();

        // when
        await openDialog();

        // then
        const dialog = screen.getByRole("dialog", { name: "Delete your account?" });
        expect(within(dialog).getByLabelText("Password")).toHaveAttribute("type", "password");
        expect(deletion.mutate).not.toHaveBeenCalled();
    });

    it("clears an earlier refusal when the question opens again", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteAccount.mockReturnValue(deletion);
        renderSection();

        // when
        await openDialog();

        // then
        expect(deletion.reset).toHaveBeenCalledOnce();
    });

    it("deletes the account with the password once confirmed", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteAccount.mockReturnValue(deletion);
        renderSection();
        const user = await openDialog();
        const dialog = screen.getByRole("dialog", { name: "Delete your account?" });

        // when
        await user.type(within(dialog).getByLabelText("Password"), "harbour-lights");
        await user.click(within(dialog).getByRole("button", { name: "Delete account" }));

        // then
        expect(deletion.mutate).toHaveBeenCalledWith(
            { password: "harbour-lights" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("asks for the password before sending anything, and moves focus to the message", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteAccount.mockReturnValue(deletion);
        renderSection();
        const user = await openDialog();
        const dialog = screen.getByRole("dialog", { name: "Delete your account?" });

        // when
        await user.click(within(dialog).getByRole("button", { name: "Delete account" }));

        // then
        expect(within(dialog).getByLabelText("Password")).toHaveAccessibleDescription("Enter your password.");
        expect(within(dialog).getByRole("alert")).toHaveTextContent("Enter your password.");
        expect(within(dialog).getByRole("alert")).toHaveFocus();
        expect(deletion.mutate).not.toHaveBeenCalled();
    });

    it("deletes the account when the password is submitted with the Enter key", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteAccount.mockReturnValue(deletion);
        renderSection();
        const user = await openDialog();

        // when
        await user.type(screen.getByLabelText("Password"), "harbour-lights{Enter}");

        // then
        expect(deletion.mutate).toHaveBeenCalledWith({ password: "harbour-lights" }, expect.anything());
    });

    it("leaves for the home page once the account is gone", async () => {
        // given
        useDeleteAccount.mockReturnValue(
            makeMutationState({
                mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
                    options?.onSuccess?.();
                }),
            }),
        );
        renderSection();
        const user = await openDialog();

        // when
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete account" }));

        // then
        expect(screen.getByText("the home page")).toBeInTheDocument();
    });

    it("keeps the account when the question is cancelled", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteAccount.mockReturnValue(deletion);
        renderSection();
        const user = await openDialog();
        await user.type(screen.getByLabelText("Password"), "harbour-lights");

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(deletion.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("forgets the typed password once the question is cancelled", async () => {
        // given
        renderSection();
        const user = await openDialog();
        await user.type(screen.getByLabelText("Password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // when
        await user.click(screen.getByRole("button", { name: "Delete account" }));

        // then
        expect(screen.getByLabelText("Password")).toHaveValue("");
    });

    it("holds the question while the account is being deleted", async () => {
        // given
        useDeleteAccount.mockReturnValue(makeMutationState({ isPending: true }));
        renderSection();

        // when
        await openDialog();

        // then
        const dialog = screen.getByRole("dialog", { name: "Delete your account?" });
        expect(within(dialog).getByRole("button", { name: "Delete account" })).toBeDisabled();
        expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    });

    it("ties a wrong password to the field and says why", async () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "the current password is incorrect",
            code: "incorrect_password",
        });
        useDeleteAccount.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        const dialog = screen.getByRole("dialog", { name: "Delete your account?" });
        expect(within(dialog).getByRole("alert")).toHaveTextContent("the current password is incorrect");
        expect(within(dialog).getByLabelText("Password")).toHaveAttribute("aria-invalid", "true");
        expect(within(dialog).getByLabelText("Password")).toHaveAccessibleDescription(
            "the current password is incorrect",
        );
    });

    it("shows any other refusal without blaming the password", async () => {
        // given
        const failure = makeApiError({ status: 403, message: "your account is locked" });
        useDeleteAccount.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        const dialog = screen.getByRole("dialog", { name: "Delete your account?" });
        expect(within(dialog).getByRole("alert")).toHaveTextContent("your account is locked");
        expect(within(dialog).getByLabelText("Password")).not.toHaveAttribute("aria-invalid");
    });

    it("asks an account with no password to set one first", () => {
        // given
        const user = makeSessionUser({ has_password: false });

        // when
        renderSection(user);

        // then
        expect(screen.getByText("Set a password before you delete your account.")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Delete account" })).not.toBeInTheDocument();
    });
});
