import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeAdminPermissions,
    makeApiError,
    makeMutationState,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminPermissionsResponse } from "../../../types/api";
import { AdminPermissionsPage } from "./AdminPermissionsPage";

const { useAdminPermissions, useUpdateRolePermissions } = vi.hoisted(() => ({
    useAdminPermissions: vi.fn(),
    useUpdateRolePermissions: vi.fn(),
}));

vi.mock("../../../hooks/queries/admin", () => ({ useAdminPermissions }));
vi.mock("../../../hooks/mutations/admin", () => ({ useUpdateRolePermissions }));

function renderPage() {
    return renderWithProviders(<AdminPermissionsPage />, {
        user: makeSessionUser(),
        permissions: ["view_admin_panel", "manage_roles"],
    });
}

beforeEach(() => {
    useAdminPermissions.mockReturnValue(makeQueryState<AdminPermissionsResponse>({ data: makeAdminPermissions() }));
    useUpdateRolePermissions.mockReturnValue(makeMutationState());
});

describe("AdminPermissionsPage", () => {
    it("heads the page and says which roles cannot be edited", () => {
        // given
        const title = "Permissions";

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: title })).toBeInTheDocument();
        expect(
            screen.getByText(/Super admins and admins can do everything\. Support is a limited staff role/),
        ).toBeInTheDocument();
    });

    it("offers a form for each role that can be edited", () => {
        // given
        const response = makeAdminPermissions();

        // when
        renderPage();

        // then
        for (const role of response.roles) {
            expect(screen.getByRole("region", { name: role.label })).toBeInTheDocument();
        }
        expect(screen.getByRole("group", { name: "Support permissions" })).toBeInTheDocument();
    });

    it("says it is loading while the permissions are on their way", () => {
        // given
        useAdminPermissions.mockReturnValue(makeQueryState<AdminPermissionsResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading permissions...")).toBeInTheDocument();
    });

    it("shows the server's reason when the permissions could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<AdminPermissionsResponse>({
            error: makeApiError({ status: 500, message: "failed to load the permissions" }),
        });
        useAdminPermissions.mockReturnValue(failed);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to load the permissions");
        expect(screen.queryByRole("switch")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
