import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { DeleteUserSection } from "./DeleteUserSection";

const { useDeleteUser } = vi.hoisted(() => ({ useDeleteUser: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useDeleteUser }));

const robin = makeAdminUserDetail({ id: "user-robin", display_name: "Robin" });

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

function renderSection() {
    return renderWithProviders(
        <Routes>
            <Route path="/admin/users/:userId" element={<DeleteUserSection user={robin} />} />
            <Route path="/admin/users" element={<p>the users page</p>} />
        </Routes>,
        { route: "/admin/users/user-robin" },
    );
}

beforeEach(() => {
    useDeleteUser.mockReturnValue(makeMutationState());
});

describe("DeleteUserSection", () => {
    it("asks before deleting, then deletes and returns to the user list", async () => {
        // given
        const deleteUser = makeMutationState({ mutate: vi.fn(succeed) });
        useDeleteUser.mockReturnValue(deleteUser);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Delete user" }));
        const dialog = screen.getByRole("dialog", { name: "Delete user" });
        const warning = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Delete user" }));

        // then
        expect(warning).toContain("Robin's account and everything in it is deleted. This cannot be undone.");
        expect(deleteUser.mutate).toHaveBeenCalledWith({ userId: "user-robin" }, expect.anything());
        expect(screen.getByText("the users page")).toBeInTheDocument();
    });

    it("keeps the user when cancelled", async () => {
        // given
        const deleteUser = makeMutationState();
        useDeleteUser.mockReturnValue(deleteUser);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Delete user" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(deleteUser.mutate).not.toHaveBeenCalled();
        expect(screen.queryByText("the users page")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when the deletion fails", async () => {
        // given
        const error = makeApiError({ status: 403, message: "this user cannot be modified" });
        useDeleteUser.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Delete user" }));

        // then
        expect(screen.getByRole("dialog", { name: "Delete user" })).toHaveTextContent("this user cannot be modified");
        expect(screen.queryByText("the users page")).not.toBeInTheDocument();
    });

    it("holds the dialog's buttons while the deletion runs", async () => {
        // given
        useDeleteUser.mockReturnValue(makeMutationState({ isPending: true }));
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Delete user" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "Delete user" });
        expect(within(dialog).getByRole("button", { name: "Delete user" })).toBeDisabled();
        expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    });
});
