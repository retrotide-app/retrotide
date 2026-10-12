import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { SignOutSection } from "./SignOutSection";

const { useForceLogoutUser } = vi.hoisted(() => ({ useForceLogoutUser: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useForceLogoutUser }));

const robin = makeAdminUserDetail({ id: "user-robin", display_name: "Robin" });

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    useForceLogoutUser.mockReturnValue(makeMutationState());
});

describe("SignOutSection", () => {
    it("asks before signing the user out everywhere, then does it", async () => {
        // given
        const signOut = makeMutationState({ mutate: vi.fn(succeed) });
        useForceLogoutUser.mockReturnValue(signOut);
        const user = userEvent.setup();
        renderWithProviders(<SignOutSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Sign out everywhere" }));
        const dialog = screen.getByRole("dialog", { name: "Sign out everywhere" });
        await user.click(within(dialog).getByRole("button", { name: "Sign out everywhere" }));

        // then
        expect(dialog).toHaveTextContent("Robin is signed out on every device. Their password stays the same.");
        expect(signOut.mutate).toHaveBeenCalledWith({ userId: "user-robin" }, expect.anything());
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("does nothing when cancelled", async () => {
        // given
        const signOut = makeMutationState();
        useForceLogoutUser.mockReturnValue(signOut);
        const user = userEvent.setup();
        renderWithProviders(<SignOutSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Sign out everywhere" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(signOut.mutate).not.toHaveBeenCalled();
    });

    it("keeps the check open with the server's reason when it fails", async () => {
        // given
        const error = makeApiError({ status: 500, message: "failed to sign the user out" });
        useForceLogoutUser.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderWithProviders(<SignOutSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Sign out everywhere" }));

        // then
        expect(screen.getByRole("dialog", { name: "Sign out everywhere" })).toHaveTextContent(
            "failed to sign the user out",
        );
    });

    it("says it is done", () => {
        // given
        useForceLogoutUser.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<SignOutSection user={robin} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Signed out everywhere.");
    });
});
