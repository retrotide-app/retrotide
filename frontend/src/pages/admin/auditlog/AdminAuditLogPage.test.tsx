import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../../domain/permissions";
import {
    makeApiError,
    makeAuditLogEntry,
    makeAuditLogList,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AuditLogListResponse } from "../../../types/api";
import { AdminAuditLogPage } from "./AdminAuditLogPage";

const { useAuditLog } = vi.hoisted(() => ({ useAuditLog: vi.fn() }));

vi.mock("../../../hooks/queries/admin", () => ({ useAuditLog }));

const banEntry = makeAuditLogEntry({
    id: 7,
    action: "ban_user",
    actor_name: "Sam",
    subject_id: "user-robin",
    subject_name: "Robin",
    details: { reason: "spam" },
});

function renderPage(permissions: StaffPermission[] = ["view_admin_panel", "view_audit_log", "view_users"]) {
    return renderWithProviders(<AdminAuditLogPage />, { user: makeSessionUser(), permissions });
}

beforeEach(() => {
    vi.clearAllMocks();
    useAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([banEntry]) }));
});

describe("AdminAuditLogPage", () => {
    it("asks for the first page of every action", () => {
        // given
        const expected = { action: undefined, limit: 50, offset: 0 };

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Audit log" })).toBeInTheDocument();
        expect(useAuditLog).toHaveBeenLastCalledWith(expected);
    });

    it("lists each entry with what happened, to whom, by whom and why", () => {
        // given
        const listing = makeAuditLogList([banEntry]);
        useAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: listing }));

        // when
        renderPage();

        // then
        const [, row] = within(screen.getByRole("table", { name: "Audit log" })).getAllByRole("row");
        expect(row).toHaveTextContent("User banned");
        expect(row).toHaveTextContent("Robin");
        expect(row).toHaveTextContent("Sam");
        expect(row).toHaveTextContent("spam");
    });

    it("links each user to their record for a viewer who may open user records", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_audit_log", "view_users"];

        // when
        renderPage(permissions);

        // then
        expect(screen.getByRole("link", { name: "Robin" })).toHaveAttribute("href", "/admin/users/user-robin");
    });

    it("names users without links for a viewer who may not open user records", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_audit_log"];

        // when
        renderPage(permissions);

        // then
        expect(screen.getByText("Robin")).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Robin" })).not.toBeInTheDocument();
    });

    it("offers only the actions the site log records", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_audit_log"];

        // when
        renderPage(permissions);

        // then
        const filter = screen.getByRole("combobox", { name: "Action" });
        const options = within(filter).getAllByRole("option");
        expect(options[0]).toHaveTextContent("All actions");
        expect(within(filter).getByRole("option", { name: "User banned" })).toBeInTheDocument();
        expect(within(filter).queryByRole("option", { name: "Board created" })).not.toBeInTheDocument();
    });

    it("filters by action from the first page", async () => {
        // given
        useAuditLog.mockReturnValue(
            makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([banEntry], { total: 120 }) }),
        );
        const user = userEvent.setup();
        renderPage();
        await user.click(screen.getByRole("button", { name: "Next" }));

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Action" }), "lock_user");

        // then
        expect(useAuditLog).toHaveBeenLastCalledWith({ action: "lock_user", limit: 50, offset: 0 });
    });

    it("goes back to every action", async () => {
        // given
        const user = userEvent.setup();
        renderPage();
        await user.selectOptions(screen.getByRole("combobox", { name: "Action" }), "lock_user");

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Action" }), "");

        // then
        expect(useAuditLog).toHaveBeenLastCalledWith({ action: undefined, limit: 50, offset: 0 });
    });

    it("moves through the pages", async () => {
        // given
        useAuditLog.mockReturnValue(
            makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([banEntry], { total: 120 }) }),
        );
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(useAuditLog).toHaveBeenLastCalledWith({ action: undefined, limit: 50, offset: 50 });
    });

    it("says it is loading while the entries are on their way", () => {
        // given
        useAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading the audit log...")).toBeInTheDocument();
    });

    it("says so when nothing has been recorded", () => {
        // given
        useAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([]) }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Nothing recorded yet.")).toBeInTheDocument();
    });

    it("says so when no entry matches the filter", async () => {
        // given
        useAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([]) }));
        const user = userEvent.setup();
        renderPage();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Action" }), "lock_user");

        // then
        expect(screen.getByText("No entries match.")).toBeInTheDocument();
    });

    it("shows the server's reason when the entries could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<AuditLogListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the audit log" }),
        });
        useAuditLog.mockReturnValue(failed);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the audit log");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
