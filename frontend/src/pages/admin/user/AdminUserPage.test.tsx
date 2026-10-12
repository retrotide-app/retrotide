import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../../domain/permissions";
import {
    makeAdminUserDetail,
    makeApiError,
    makeAuditLogList,
    makeMutationState,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminUserDetailResponse, AuditLogListResponse } from "../../../types/api";
import { AdminUserPage } from "./AdminUserPage";

const queries = vi.hoisted(() => ({ useAdminUser: vi.fn(), useAdminUserAuditLog: vi.fn() }));

const mutations = vi.hoisted(() => ({
    useSetUserDisplayName: vi.fn(),
    useSetUserEmail: vi.fn(),
    useVerifyUserEmail: vi.fn(),
    useUnverifyUserEmail: vi.fn(),
    useSetUserRole: vi.fn(),
    useRemoveUserRole: vi.fn(),
    useBanUser: vi.fn(),
    useUnbanUser: vi.fn(),
    useLockUser: vi.fn(),
    useUnlockUser: vi.fn(),
    useForceLogoutUser: vi.fn(),
    useResetUserPassword: vi.fn(),
    useDeleteUser: vi.fn(),
}));

vi.mock("../../../hooks/queries/admin", () => queries);
vi.mock("../../../hooks/mutations/admin", () => mutations);

const viewer = makeSessionUser({ id: "user-sam", display_name: "Sam" });
const robin = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", email: "robin@example.test" });

const everyPermission: StaffPermission[] = [
    "view_admin_panel",
    "view_audit_log",
    "manage_roles",
    "view_users",
    "ban_user",
    "manage_user_account",
    "manage_user_email",
    "set_email_verified",
    "reset_password",
    "delete_any_user",
];

function renderPage(permissions: StaffPermission[] = everyPermission) {
    return renderWithProviders(<AdminUserPage />, {
        user: viewer,
        permissions,
        route: "/admin/users/user-robin",
        path: "/admin/users/:userId",
    });
}

function showUser(user: AdminUserDetailResponse) {
    queries.useAdminUser.mockReturnValue(makeQueryState<AdminUserDetailResponse>({ data: user }));
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showUser(robin);
    queries.useAdminUserAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([]) }));
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
});

describe("AdminUserPage", () => {
    it("loads the user named in the address", () => {
        // given
        const expectedId = "user-robin";

        // when
        renderPage();

        // then
        expect(queries.useAdminUser).toHaveBeenLastCalledWith(expectedId);
    });

    it("says it is loading while the user is on the way", () => {
        // given
        queries.useAdminUser.mockReturnValue(makeQueryState<AdminUserDetailResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading user...")).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to users" })).toHaveAttribute("href", "/admin/users");
    });

    it("shows the server's reason when the user could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<AdminUserDetailResponse>({
            error: makeApiError({ status: 404, message: "user not found" }),
        });
        queries.useAdminUser.mockReturnValue(failed);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("user not found");
        expect(screen.queryByText("Loading user...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("heads the page with the user's name and shows their details", () => {
        // given
        const user = robin;

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: user.display_name })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Details" })).toHaveTextContent("robin@example.test");
        expect(document.title).toContain("Robin");
    });

    it("still shows the record, with the reason, when a refresh fails", () => {
        // given
        queries.useAdminUser.mockReturnValue(
            makeQueryState<AdminUserDetailResponse>({
                data: robin,
                error: makeApiError({ status: 500, message: "failed to load the user" }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to load the user");
        expect(screen.getByRole("region", { name: "Details" })).toBeInTheDocument();
    });

    it("offers every action the viewer's permissions allow", () => {
        // given
        const permissions = everyPermission;

        // when
        renderPage(permissions);

        // then
        for (const name of [
            "Display name",
            "Email address",
            "Role",
            "Ban",
            "Lock",
            "Sessions",
            "Password",
            "Delete user",
            "Audit log",
        ]) {
            expect(screen.getByRole("region", { name })).toBeInTheDocument();
        }
    });

    it("offers only the record to a viewer who may only read user records", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_users"];

        // when
        renderPage(permissions);

        // then
        expect(screen.getByRole("region", { name: "Details" })).toBeInTheDocument();
        for (const name of [
            "Display name",
            "Email address",
            "Role",
            "Ban",
            "Lock",
            "Sessions",
            "Password",
            "Delete user",
            "Audit log",
        ]) {
            expect(screen.queryByRole("region", { name })).not.toBeInTheDocument();
        }
    });

    it.each<[StaffPermission, string[]]>([
        ["manage_user_account", ["Display name"]],
        ["manage_user_email", ["Email address"]],
        ["set_email_verified", ["Email address"]],
        ["manage_roles", ["Role"]],
        ["ban_user", ["Ban", "Lock", "Sessions"]],
        ["reset_password", ["Password"]],
        ["delete_any_user", ["Delete user"]],
        ["view_audit_log", ["Audit log"]],
    ])("opens the %s actions and no others", (permission, regions) => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_users", permission];

        // when
        renderPage(permissions);

        // then
        for (const name of regions) {
            expect(screen.getByRole("region", { name })).toBeInTheDocument();
        }
        const others = ["Display name", "Email address", "Role", "Ban", "Password", "Delete user", "Audit log"];
        for (const name of others.filter(region => !regions.includes(region))) {
            expect(screen.queryByRole("region", { name })).not.toBeInTheDocument();
        }
    });

    it("lets a viewer who may only change the address do nothing about verification", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_users", "manage_user_email"];

        // when
        renderPage(permissions);

        // then
        expect(screen.getByRole("button", { name: "Change email address" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Mark unverified" })).not.toBeInTheDocument();
    });

    it("offers no actions on the viewer's own account", () => {
        // given
        showUser(makeAdminUserDetail({ id: viewer.id, display_name: "Sam", role: "admin" }));

        // when
        renderPage();

        // then
        expect(screen.getByText(/This is your own account/)).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "account settings" })).toHaveAttribute("href", "/account");
        expect(screen.queryByRole("region", { name: "Ban" })).not.toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Delete user" })).not.toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Audit log" })).toBeInTheDocument();
    });

    it("offers no actions on a super admin", () => {
        // given
        showUser(makeAdminUserDetail({ id: "user-root", display_name: "Root", role: "super_admin" }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Nobody can change a super admin's account here.")).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Role" })).not.toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Password" })).not.toBeInTheDocument();
    });

    it("offers a support member no actions on other staff", () => {
        // given
        showUser(makeAdminUserDetail({ id: "user-kit", display_name: "Kit", role: "support" }));
        const permissions: StaffPermission[] = ["view_admin_panel", "view_users", "ban_user", "reset_password"];

        // when
        renderPage(permissions);

        // then
        expect(screen.getByText("Only an admin can change another staff member's account.")).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Ban" })).not.toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Password" })).not.toBeInTheDocument();
    });

    it("lets an admin act on other staff, leaving the rank check to the server", () => {
        // given
        showUser(makeAdminUserDetail({ id: "user-kit", display_name: "Kit", role: "admin" }));

        // when
        renderPage(everyPermission);

        // then
        expect(screen.getByRole("region", { name: "Role" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Ban" })).toBeInTheDocument();
    });
});
