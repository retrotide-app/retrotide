import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminPermissions, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { RolePermissionsForm } from "./RolePermissionsForm";

const { useUpdateRolePermissions } = vi.hoisted(() => ({ useUpdateRolePermissions: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useUpdateRolePermissions }));

const { permissions: catalogue, roles } = makeAdminPermissions();
const support = { ...roles[0], permissions: ["view_admin_panel", "view_users"] };

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    useUpdateRolePermissions.mockReturnValue(makeMutationState());
});

describe("RolePermissionsForm", () => {
    it("shows every permission the role may be given, switched on for the ones it holds", () => {
        // given
        const role = support;

        // when
        renderWithProviders(<RolePermissionsForm role={role} catalogue={catalogue} />);

        // then
        const switches = screen.getAllByRole("switch");
        expect(switches).toHaveLength(catalogue.length);
        expect(screen.getByRole("switch", { name: "View user records" })).toBeChecked();
        expect(screen.getByRole("switch", { name: "Ban and lock users" })).not.toBeChecked();
        expect(screen.getByRole("region", { name: "Support" })).toBeInTheDocument();
    });

    it("holds Save permissions until something changes", () => {
        // given
        const role = support;

        // when
        renderWithProviders(<RolePermissionsForm role={role} catalogue={catalogue} />);

        // then
        expect(screen.getByRole("button", { name: "Save permissions" })).toBeDisabled();
    });

    it("saves the role's new permissions", async () => {
        // given
        const update = makeMutationState({ mutate: vi.fn(succeed) });
        useUpdateRolePermissions.mockReturnValue(update);
        const user = userEvent.setup();
        renderWithProviders(<RolePermissionsForm role={support} catalogue={catalogue} />);

        // when
        await user.click(screen.getByRole("switch", { name: "Ban and lock users" }));
        await user.click(screen.getByRole("switch", { name: "View user records" }));
        await user.click(screen.getByRole("button", { name: "Save permissions" }));

        // then
        expect(update.mutate).toHaveBeenCalledWith(
            { role: "support", permissions: ["view_admin_panel", "ban_user"] },
            expect.anything(),
        );
    });

    it("counts switching a permission back as no change", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<RolePermissionsForm role={support} catalogue={catalogue} />);
        const toggle = screen.getByRole("switch", { name: "Ban and lock users" });

        // when
        await user.click(toggle);
        await user.click(toggle);

        // then
        expect(screen.getByRole("button", { name: "Save permissions" })).toBeDisabled();
    });

    it("says the permissions were saved", () => {
        // given
        useUpdateRolePermissions.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<RolePermissionsForm role={support} catalogue={catalogue} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Permissions saved.");
    });

    it("holds the switches and the button while saving", () => {
        // given
        useUpdateRolePermissions.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<RolePermissionsForm role={support} catalogue={catalogue} />);

        // then
        expect(screen.getByRole("switch", { name: "View user records" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Save permissions" })).toBeDisabled();
    });

    it("shows the server's reason when the permissions were refused, and moves focus to it", () => {
        // given
        const error = makeApiError({
            status: 400,
            message: 'this permission is reserved for admins and cannot be granted to another role: "manage_roles"',
        });
        useUpdateRolePermissions.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderWithProviders(<RolePermissionsForm role={support} catalogue={catalogue} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("this permission is reserved for admins");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
