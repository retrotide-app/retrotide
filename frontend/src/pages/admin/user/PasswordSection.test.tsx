import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminPasswordResetResponse } from "../../../types/api";
import { PasswordSection } from "./PasswordSection";

const { useResetUserPassword } = vi.hoisted(() => ({ useResetUserPassword: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useResetUserPassword }));

const robin = makeAdminUserDetail({ id: "user-robin", display_name: "Robin" });

function resetWith(password: string) {
    return (_variables: unknown, options?: { onSuccess?: (result: AdminPasswordResetResponse) => void }) => {
        options?.onSuccess?.({ password });
    };
}

beforeEach(() => {
    useResetUserPassword.mockReturnValue(makeMutationState());
});

describe("PasswordSection", () => {
    it("asks before resetting the password", async () => {
        // given
        const resetPassword = makeMutationState();
        useResetUserPassword.mockReturnValue(resetPassword);
        const user = userEvent.setup();
        renderWithProviders(<PasswordSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Reset password" }));

        // then
        expect(screen.getByRole("dialog", { name: "Reset password" })).toHaveTextContent(
            "Robin's password is replaced with a new one and they are signed out everywhere. You see the new password once.",
        );
        expect(resetPassword.mutate).not.toHaveBeenCalled();
    });

    it("resets the password and shows the new one in a field that cannot be edited", async () => {
        // given
        const resetPassword = makeMutationState({ mutate: vi.fn(resetWith("CORRECT-HORSE-7")) });
        useResetUserPassword.mockReturnValue(resetPassword);
        const user = userEvent.setup();
        renderWithProviders(<PasswordSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Reset password" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Reset password" })).getByRole("button", {
                name: "Reset password",
            }),
        );

        // then
        const shown = screen.getByRole("dialog", { name: "New password" });
        const field = within(shown).getByRole("textbox", { name: "New password" });
        expect(resetPassword.mutate).toHaveBeenCalledWith({ userId: "user-robin" }, expect.anything());
        expect(shown).toHaveTextContent("Give this password to Robin. It is not shown again.");
        expect(field).toHaveValue("CORRECT-HORSE-7");
        expect(field).toHaveAttribute("readonly");
        expect(screen.queryByRole("dialog", { name: "Reset password" })).not.toBeInTheDocument();
    });

    it("forgets the new password once it has been noted", async () => {
        // given
        const resetPassword = makeMutationState({ mutate: vi.fn(resetWith("CORRECT-HORSE-7")) });
        useResetUserPassword.mockReturnValue(resetPassword);
        const user = userEvent.setup();
        renderWithProviders(<PasswordSection user={robin} />);
        await user.click(screen.getByRole("button", { name: "Reset password" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Reset password" })).getByRole("button", {
                name: "Reset password",
            }),
        );
        resetPassword.reset.mockClear();

        // when
        await user.click(screen.getByRole("button", { name: "Done" }));

        // then
        expect(screen.queryByRole("dialog", { name: "New password" })).not.toBeInTheDocument();
        expect(screen.queryByDisplayValue("CORRECT-HORSE-7")).not.toBeInTheDocument();
        expect(resetPassword.reset).toHaveBeenCalledOnce();
    });

    it("does nothing when cancelled", async () => {
        // given
        const resetPassword = makeMutationState();
        useResetUserPassword.mockReturnValue(resetPassword);
        const user = userEvent.setup();
        renderWithProviders(<PasswordSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Reset password" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(resetPassword.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when the reset fails", async () => {
        // given
        const error = makeApiError({ status: 403, message: "this user cannot be modified" });
        useResetUserPassword.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderWithProviders(<PasswordSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Reset password" }));

        // then
        expect(screen.getByRole("dialog", { name: "Reset password" })).toHaveTextContent(
            "this user cannot be modified",
        );
        expect(screen.queryByRole("dialog", { name: "New password" })).not.toBeInTheDocument();
    });
});
