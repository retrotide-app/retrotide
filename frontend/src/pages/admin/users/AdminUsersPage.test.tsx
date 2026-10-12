import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeAdminUser,
    makeAdminUserList,
    makeApiError,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminUserListResponse } from "../../../types/api";
import { AdminUsersPage } from "./AdminUsersPage";

const { useAdminUsers } = vi.hoisted(() => ({ useAdminUsers: vi.fn() }));

vi.mock("../../../hooks/queries/admin", () => ({ useAdminUsers }));

const robin = makeAdminUser({ id: "user-robin", display_name: "Robin", email: "robin@example.test" });

function renderPage() {
    return renderWithProviders(<AdminUsersPage />, {
        user: makeSessionUser(),
        permissions: ["view_admin_panel", "view_users"],
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    useAdminUsers.mockReturnValue(makeQueryState<AdminUserListResponse>({ data: makeAdminUserList([robin]) }));
});

describe("AdminUsersPage", () => {
    it("heads the page", () => {
        // given
        const title = "Users";

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: title })).toBeInTheDocument();
    });

    it("asks for the first page of every user", () => {
        // given
        const expected = { search: "", limit: 20, offset: 0 };

        // when
        renderPage();

        // then
        expect(useAdminUsers).toHaveBeenLastCalledWith(expected);
    });

    it("says it is loading while the first page is on its way", () => {
        // given
        useAdminUsers.mockReturnValue(makeQueryState<AdminUserListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading users...")).toBeInTheDocument();
    });

    it("lists each user with a link to their record, their role and their status", () => {
        // given
        const users = [
            robin,
            makeAdminUser({
                id: "user-kit",
                display_name: "Kit",
                email: "kit@example.test",
                role: "support",
                banned: true,
                locked: true,
                email_verified: false,
            }),
        ];
        useAdminUsers.mockReturnValue(makeQueryState<AdminUserListResponse>({ data: makeAdminUserList(users) }));

        // when
        renderPage();

        // then
        const table = screen.getByRole("table", { name: "Users" });
        const [, robinRow, kitRow] = within(table).getAllByRole("row");
        expect(within(robinRow).getByRole("link", { name: "Robin" })).toHaveAttribute(
            "href",
            "/admin/users/user-robin",
        );
        expect(robinRow).toHaveTextContent("robin@example.test");
        expect(robinRow).toHaveTextContent("No role");
        expect(robinRow).toHaveTextContent("Active");
        expect(kitRow).toHaveTextContent("Support");
        expect(kitRow).toHaveTextContent("Banned");
        expect(kitRow).toHaveTextContent("Locked");
        expect(kitRow).toHaveTextContent("Unverified");
        expect(kitRow).not.toHaveTextContent("Active");
    });

    it("searches for what was typed, from the first page", async () => {
        // given
        const user = userEvent.setup();
        useAdminUsers.mockReturnValue(
            makeQueryState<AdminUserListResponse>({ data: makeAdminUserList([robin], { total: 45 }) }),
        );
        renderPage();
        await user.click(screen.getByRole("button", { name: "Next" }));

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search users" }), "  robin ");
        await user.click(screen.getByRole("button", { name: "Search" }));

        // then
        expect(useAdminUsers).toHaveBeenLastCalledWith({ search: "robin", limit: 20, offset: 0 });
    });

    it("moves through the pages", async () => {
        // given
        const user = userEvent.setup();
        useAdminUsers.mockReturnValue(
            makeQueryState<AdminUserListResponse>({ data: makeAdminUserList([robin], { total: 45 }) }),
        );
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(useAdminUsers).toHaveBeenLastCalledWith({ search: "", limit: 20, offset: 20 });
    });

    it("says so when there are no users", () => {
        // given
        useAdminUsers.mockReturnValue(makeQueryState<AdminUserListResponse>({ data: makeAdminUserList([]) }));

        // when
        renderPage();

        // then
        expect(screen.getByText("No users yet.")).toBeInTheDocument();
        expect(screen.queryByRole("table", { name: "Users" })).not.toBeInTheDocument();
    });

    it("says so when nobody matches the search", async () => {
        // given
        const user = userEvent.setup();
        useAdminUsers.mockReturnValue(makeQueryState<AdminUserListResponse>({ data: makeAdminUserList([]) }));
        renderPage();

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search users" }), "nobody");
        await user.click(screen.getByRole("button", { name: "Search" }));

        // then
        expect(screen.getByText("No users match that search.")).toBeInTheDocument();
    });

    it("shows the server's reason when the users could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<AdminUserListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the users" }),
        });
        useAdminUsers.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the users");
        expect(screen.queryByText("Loading users...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
