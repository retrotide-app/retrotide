import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeAdminUserDetail,
    makeApiError,
    makeAuditLogEntry,
    makeAuditLogList,
    makeQueryState,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AuditLogListResponse } from "../../../types/api";
import { UserAuditLogSection } from "./UserAuditLogSection";

const { useAdminUserAuditLog } = vi.hoisted(() => ({ useAdminUserAuditLog: vi.fn() }));

vi.mock("../../../hooks/queries/admin", () => ({ useAdminUserAuditLog }));

const robin = makeAdminUserDetail({ id: "user-robin", display_name: "Robin" });

beforeEach(() => {
    vi.clearAllMocks();
    useAdminUserAuditLog.mockReturnValue(
        makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([makeAuditLogEntry()]) }),
    );
});

describe("UserAuditLogSection", () => {
    it("asks for the first page of the user's entries", () => {
        // given
        const user = robin;

        // when
        renderWithProviders(<UserAuditLogSection user={user} />);

        // then
        expect(useAdminUserAuditLog).toHaveBeenLastCalledWith("user-robin", { limit: 50, offset: 0 });
    });

    it("lists what was recorded about the user", () => {
        // given
        const entries = [makeAuditLogEntry({ action: "ban_user", actor_name: "Sam" })];
        useAdminUserAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: makeAuditLogList(entries) }));

        // when
        renderWithProviders(<UserAuditLogSection user={robin} />);

        // then
        const table = screen.getByRole("table", { name: "Audit log for Robin" });
        expect(table).toHaveTextContent("User banned");
        expect(table).toHaveTextContent("Sam");
        expect(screen.queryByRole("columnheader", { name: "User" })).not.toBeInTheDocument();
    });

    it("says so when nothing has been recorded", () => {
        // given
        useAdminUserAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ data: makeAuditLogList([]) }));

        // when
        renderWithProviders(<UserAuditLogSection user={robin} />);

        // then
        expect(screen.getByText("Nothing has been recorded about this user yet.")).toBeInTheDocument();
    });

    it("says it is loading while the entries are on their way", () => {
        // given
        useAdminUserAuditLog.mockReturnValue(makeQueryState<AuditLogListResponse>({ loading: true }));

        // when
        renderWithProviders(<UserAuditLogSection user={robin} />);

        // then
        expect(screen.getByText("Loading the audit log...")).toBeInTheDocument();
    });

    it("moves through the pages", async () => {
        // given
        useAdminUserAuditLog.mockReturnValue(
            makeQueryState<AuditLogListResponse>({
                data: makeAuditLogList([makeAuditLogEntry()], { total: 120 }),
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(<UserAuditLogSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(useAdminUserAuditLog).toHaveBeenLastCalledWith("user-robin", { limit: 50, offset: 50 });
    });

    it("shows the server's reason when the entries could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<AuditLogListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the user's audit log" }),
        });
        useAdminUserAuditLog.mockReturnValue(failed);
        const user = userEvent.setup();
        renderWithProviders(<UserAuditLogSection user={robin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the user's audit log");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
