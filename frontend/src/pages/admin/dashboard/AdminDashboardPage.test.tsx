import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../../domain/permissions";
import {
    makeAdminRecentSignup,
    makeAdminStats,
    makeApiError,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminStatsResponse } from "../../../types/api";
import { AdminDashboardPage } from "./AdminDashboardPage";

const { useAdminStats } = vi.hoisted(() => ({ useAdminStats: vi.fn() }));

vi.mock("../../../hooks/queries/adminStats", () => ({ useAdminStats }));

const STATS_VIEWER: StaffPermission[] = ["view_admin_panel", "view_stats", "view_users"];

function renderPage(permissions: StaffPermission[] = STATS_VIEWER) {
    return renderWithProviders(<AdminDashboardPage />, { user: makeSessionUser(), permissions });
}

beforeEach(() => {
    vi.clearAllMocks();
    useAdminStats.mockReturnValue(makeQueryState<AdminStatsResponse>({ data: makeAdminStats() }));
});

describe("AdminDashboardPage", () => {
    it("heads the page", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Dashboard" })).toBeInTheDocument();
    });

    it("asks for the stats when the viewer may see them", () => {
        // when
        renderPage();

        // then
        expect(useAdminStats).toHaveBeenLastCalledWith(true);
    });

    it("says it is loading while the stats are on their way", () => {
        // given
        useAdminStats.mockReturnValue(makeQueryState<AdminStatsResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading the site stats...")).toBeInTheDocument();
    });

    it("shows why the stats failed and tries again on request", async () => {
        // given
        const user = userEvent.setup();
        const refetch = vi.fn().mockResolvedValue(undefined);
        useAdminStats.mockReturnValue(
            makeQueryState<AdminStatsResponse>({
                error: makeApiError({ status: 500, message: "failed to load the site stats" }),
                refetch,
            }),
        );
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to load the site stats");
        expect(refetch).toHaveBeenCalledOnce();
        expect(screen.queryByText("Loading the site stats...")).not.toBeInTheDocument();
    });

    it("gives each total with how many were added in the last 7 days", () => {
        // when
        renderPage();

        // then
        const totals = screen.getByRole("region", { name: "Totals" });
        const accounts = within(totals).getByText("Accounts").parentElement;
        expect(accounts).toHaveTextContent("1,284");
        expect(accounts).toHaveTextContent("12 new in the last 7 days");
        const teams = within(totals).getByText("Teams").parentElement;
        expect(teams).toHaveTextContent("530");
        expect(teams).toHaveTextContent("None new in the last 7 days");
    });

    it("charts new accounts, boards and cards over the last 30 days", () => {
        // when
        renderPage();

        // then
        const trends = screen.getByRole("region", { name: "Last 30 days" });
        expect(within(trends).getByRole("group", { name: "New accounts, day by day" })).toBeInTheDocument();
        expect(within(trends).getByRole("group", { name: "New boards, day by day" })).toBeInTheDocument();
        expect(within(trends).getByRole("group", { name: "New cards, day by day" })).toBeInTheDocument();
    });

    it("tabulates what was created in each period", () => {
        // when
        renderPage();

        // then
        const table = screen.getByRole("table", {
            name: "New accounts, organisations, boards and cards in each period",
        });
        const cards = within(table).getByRole("row", { name: /Cards/ });
        expect(cards).toHaveTextContent("120");
        expect(cards).toHaveTextContent("902");
        expect(cards).toHaveTextContent("3,400");
    });

    it("reports the site's health", () => {
        // given
        const stats = makeAdminStats({
            health: {
                site_mode: "private",
                maintenance_mode: true,
                email_configured: false,
                email_status: "off",
                google_sign_in_configured: true,
            },
        });
        useAdminStats.mockReturnValue(makeQueryState<AdminStatsResponse>({ data: stats }));

        // when
        renderPage();

        // then
        const health = screen.getByRole("region", { name: "Site health" });
        expect(within(health).getByText("Who can join").parentElement).toHaveTextContent("Invitation only");
        expect(within(health).getByText("Maintenance mode").parentElement).toHaveTextContent("On");
        expect(within(health).getByText("Email sending").parentElement).toHaveTextContent("Not set up");
        expect(within(health).getByText("Google sign-in").parentElement).toHaveTextContent("Set up");
    });

    it("says email is set up when it is ready", () => {
        // when
        renderPage();

        // then
        const health = screen.getByRole("region", { name: "Site health" });
        expect(within(health).getByText("Email sending").parentElement).toHaveTextContent(/^Email sendingSet up$/);
    });

    it("flags email that is set up but failing apart from email that is not set up", () => {
        // given
        const stats = makeAdminStats({
            health: {
                site_mode: "public",
                maintenance_mode: false,
                email_configured: false,
                email_status: "failing",
                google_sign_in_configured: false,
            },
        });
        useAdminStats.mockReturnValue(makeQueryState<AdminStatsResponse>({ data: stats }));

        // when
        renderPage();

        // then
        const health = screen.getByRole("region", { name: "Site health" });
        const email = within(health).getByText("Email sending").parentElement;
        expect(email).toHaveTextContent("Set up but not working");
        expect(email).not.toHaveTextContent("Not set up");
    });

    it("links the health panel to the settings for someone who may change them", () => {
        // when
        renderPage([...STATS_VIEWER, "manage_settings"]);

        // then
        const health = screen.getByRole("region", { name: "Site health" });
        expect(within(health).getByRole("link", { name: "Open settings" })).toHaveAttribute("href", "/admin/settings");
    });

    it("offers no settings link to someone who may not change them", () => {
        // when
        renderPage();

        // then
        const health = screen.getByRole("region", { name: "Site health" });
        expect(within(health).queryByRole("link", { name: "Open settings" })).not.toBeInTheDocument();
    });

    it("lists the newest accounts, each linking to its user record", () => {
        // given
        const stats = makeAdminStats({
            recent_signups: [
                makeAdminRecentSignup({ id: "user-kit", display_name: "Kit", email_verified: false }),
                makeAdminRecentSignup({ id: "user-morgan", display_name: "Morgan" }),
            ],
        });
        useAdminStats.mockReturnValue(makeQueryState<AdminStatsResponse>({ data: stats }));

        // when
        renderPage();

        // then
        const newest = screen.getByRole("region", { name: "Newest accounts" });
        expect(within(newest).getByRole("link", { name: "Kit" })).toHaveAttribute("href", "/admin/users/user-kit");
        expect(within(newest).getByRole("link", { name: "Morgan" })).toHaveAttribute(
            "href",
            "/admin/users/user-morgan",
        );
        expect(within(newest).getByRole("row", { name: /Kit/ })).toHaveTextContent("Unverified");
        expect(within(newest).getByRole("link", { name: "See every user" })).toHaveAttribute("href", "/admin/users");
    });

    it("hides the newest accounts when the server leaves them out for a viewer without view_users", () => {
        // given
        const stats = makeAdminStats();
        delete stats.recent_signups;
        useAdminStats.mockReturnValue(makeQueryState<AdminStatsResponse>({ data: stats }));

        // when
        renderPage(["view_admin_panel", "view_stats"]);

        // then
        expect(screen.queryByRole("region", { name: "Newest accounts" })).not.toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Totals" })).toBeInTheDocument();
    });

    it("says so when there are no accounts yet", () => {
        // given
        useAdminStats.mockReturnValue(
            makeQueryState<AdminStatsResponse>({ data: makeAdminStats({ recent_signups: [] }) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("region", { name: "Newest accounts" })).toHaveTextContent("No accounts yet.");
    });

    it("asks for no stats and points to the pages the role opens when it cannot see them", () => {
        // when
        renderPage(["view_admin_panel", "view_users"]);

        // then
        expect(useAdminStats).toHaveBeenLastCalledWith(false);
        expect(screen.getByText("Your role does not include the site stats.")).toBeInTheDocument();
        const shortcuts = screen.getByRole("navigation", { name: "Admin pages you can open" });
        expect(within(shortcuts).getByRole("link", { name: "Users" })).toHaveAttribute("href", "/admin/users");
        expect(within(shortcuts).queryByRole("link", { name: "Dashboard" })).not.toBeInTheDocument();
        expect(screen.queryByText("Loading the site stats...")).not.toBeInTheDocument();
    });
});
