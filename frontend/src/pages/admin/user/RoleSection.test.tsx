import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { RoleSection } from "./RoleSection";

const { useSetUserRole, useRemoveUserRole } = vi.hoisted(() => ({
    useSetUserRole: vi.fn(),
    useRemoveUserRole: vi.fn(),
}));

vi.mock("../../../hooks/mutations/admin", () => ({ useSetUserRole, useRemoveUserRole }));

const member = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", role: undefined });
const support = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", role: "support" });

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    useSetUserRole.mockReturnValue(makeMutationState());
    useRemoveUserRole.mockReturnValue(makeMutationState());
});

describe("RoleSection", () => {
    it("says which role the user holds", () => {
        // given
        const user = support;

        // when
        renderWithProviders(<RoleSection user={user} />);

        // then
        expect(screen.getByRole("region", { name: "Role" })).toHaveTextContent("Current role: Support.");
        expect(screen.getByRole("combobox", { name: "Role" })).toHaveValue("support");
    });

    it("offers only the roles staff can hand out", () => {
        // given
        const user = member;

        // when
        renderWithProviders(<RoleSection user={user} />);

        // then
        const options = within(screen.getByRole("combobox", { name: "Role" })).getAllByRole("option");
        expect(options.map(option => option.textContent)).toEqual(["Support", "Admin"]);
    });

    it("gives the chosen role", async () => {
        // given
        const setRole = makeMutationState();
        useSetUserRole.mockReturnValue(setRole);
        const user = userEvent.setup();
        renderWithProviders(<RoleSection user={member} />);

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Role" }), "admin");
        await user.click(screen.getByRole("button", { name: "Set role" }));

        // then
        expect(setRole.mutate).toHaveBeenCalledWith({ userId: "user-robin", role: "admin" });
    });

    it("holds Set role while the chosen role is the one the user already holds", () => {
        // given
        const user = support;

        // when
        renderWithProviders(<RoleSection user={user} />);

        // then
        expect(screen.getByRole("button", { name: "Set role" })).toBeDisabled();
    });

    it("says which role was set", () => {
        // given
        useSetUserRole.mockReturnValue(
            makeMutationState({ isSuccess: true, variables: { userId: "user-robin", role: "admin" } }),
        );

        // when
        renderWithProviders(<RoleSection user={member} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Role set to Admin.");
    });

    it("shows the server's reason when the role was refused", () => {
        // given
        const error = makeApiError({ status: 403, message: "cannot grant a role equal to or above your own" });
        useSetUserRole.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderWithProviders(<RoleSection user={member} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("cannot grant a role equal to or above your own");
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("offers no removal to a user with no role", () => {
        // given
        const user = member;

        // when
        renderWithProviders(<RoleSection user={user} />);

        // then
        expect(screen.queryByRole("button", { name: "Remove role" })).not.toBeInTheDocument();
    });

    it("asks before removing the user's role, then removes it", async () => {
        // given
        const removeRole = makeMutationState({ mutate: vi.fn(succeed) });
        useRemoveUserRole.mockReturnValue(removeRole);
        const user = userEvent.setup();
        renderWithProviders(<RoleSection user={support} />);

        // when
        await user.click(screen.getByRole("button", { name: "Remove role" }));
        const dialog = screen.getByRole("dialog", { name: "Remove role" });
        await user.click(within(dialog).getByRole("button", { name: "Remove role" }));

        // then
        expect(dialog).toHaveTextContent("Robin loses the Support role and every permission it gives.");
        expect(removeRole.mutate).toHaveBeenCalledWith({ userId: "user-robin", role: "support" }, expect.anything());
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the role when the removal is cancelled", async () => {
        // given
        const removeRole = makeMutationState();
        useRemoveUserRole.mockReturnValue(removeRole);
        const user = userEvent.setup();
        renderWithProviders(<RoleSection user={support} />);

        // when
        await user.click(screen.getByRole("button", { name: "Remove role" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(removeRole.mutate).not.toHaveBeenCalled();
    });

    it("keeps the check open with the server's reason when the removal fails", async () => {
        // given
        const error = makeApiError({ status: 404, message: "the user does not hold that role" });
        useRemoveUserRole.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderWithProviders(<RoleSection user={support} />);

        // when
        await user.click(screen.getByRole("button", { name: "Remove role" }));

        // then
        expect(screen.getByRole("dialog", { name: "Remove role" })).toHaveTextContent(
            "the user does not hold that role",
        );
    });

    it("says the role was removed", () => {
        // given
        useRemoveUserRole.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<RoleSection user={member} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Role removed.");
    });
});
