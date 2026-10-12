import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../../domain/permissions";
import { makeMutationState, makeOrganisation, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { OrganisationResponse, SiteMode } from "../../../types/api";
import { StaffConsoleLayout } from "./StaffConsoleLayout";

const { useLogout } = vi.hoisted(() => ({ useLogout: vi.fn() }));

vi.mock("../../../hooks/mutations/auth", () => ({ useLogout }));

interface RenderOptions {
    permissions?: StaffPermission[];
    route?: string;
    siteMode?: SiteMode;
    organisations?: OrganisationResponse[];
}

function renderConsole({
    permissions = ["view_admin_panel", "view_users"],
    route = "/admin/users",
    siteMode = "public",
    organisations = [],
}: RenderOptions = {}) {
    return renderWithProviders(
        <Routes>
            <Route path="/admin" element={<StaffConsoleLayout />}>
                <Route index element={<p>the dashboard</p>} />
                <Route path="users" element={<p>the users page</p>} />
                <Route path="settings" element={<p>the settings page</p>} />
            </Route>
        </Routes>,
        {
            user: makeSessionUser(),
            permissions,
            route,
            siteInfo: { site_mode: siteMode, site_name: "RetroTide" },
            org: { organisations },
        },
    );
}

function sections() {
    return screen.getByRole("navigation", { name: "Admin sections" });
}

beforeEach(() => {
    useLogout.mockReturnValue(makeMutationState());
});

describe("StaffConsoleLayout", () => {
    it("draws the page it wraps in the console's main area", () => {
        // when
        renderConsole();

        // then
        expect(screen.getByRole("main")).toHaveTextContent("the users page");
    });

    it("says it is the admin console and links its name to the dashboard", () => {
        // when
        renderConsole();

        // then
        const banner = screen.getByRole("banner");
        expect(within(banner).getByRole("link", { name: "RetroTide Admin console" })).toHaveAttribute("href", "/admin");
        expect(screen.getByRole("heading", { level: 1, name: "Admin console" })).toBeInTheDocument();
    });

    it("has no member sidebar and no tabs", () => {
        // when
        renderConsole();

        // then
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
        expect(screen.getByRole("complementary", { name: "Admin console" })).toBeInTheDocument();
    });

    it("offers only the pages the viewer's permissions open", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "view_users", "view_audit_log"];

        // when
        renderConsole({ permissions });

        // then
        const nav = sections();
        expect(within(nav).getByRole("link", { name: "Dashboard" })).toHaveAttribute("href", "/admin");
        expect(within(nav).getByRole("link", { name: "Users" })).toHaveAttribute("href", "/admin/users");
        expect(within(nav).getByRole("link", { name: "Audit log" })).toHaveAttribute("href", "/admin/audit-log");
        expect(within(nav).queryByRole("link", { name: "Settings" })).not.toBeInTheDocument();
        expect(within(nav).queryByRole("link", { name: "Permissions" })).not.toBeInTheDocument();
        expect(within(nav).queryByRole("link", { name: "Organisations" })).not.toBeInTheDocument();
    });

    it("groups every page for someone who holds every permission", () => {
        // given
        const permissions: StaffPermission[] = [
            "view_admin_panel",
            "view_users",
            "view_organisations",
            "manage_user_account",
            "manage_roles",
            "manage_settings",
            "view_audit_log",
        ];

        // when
        renderConsole({ permissions, siteMode: "private" });

        // then
        const accounts = within(sections()).getByRole("list", { name: "Accounts" });
        const site = within(sections()).getByRole("list", { name: "Site" });
        expect(
            within(accounts)
                .getAllByRole("link")
                .map(link => link.textContent),
        ).toEqual(["Users", "Organisations", "Invitations"]);
        expect(
            within(site)
                .getAllByRole("link")
                .map(link => link.textContent),
        ).toEqual(["Permissions", "Settings", "Audit log"]);
        expect(within(sections()).getByRole("link", { name: "Organisations" })).toHaveAttribute(
            "href",
            "/admin/organisations",
        );
    });

    it("offers the invitations on a private site to someone who may manage user accounts", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "manage_user_account"];

        // when
        renderConsole({ permissions, siteMode: "private" });

        // then
        expect(within(sections()).getByRole("link", { name: "Invitations" })).toHaveAttribute(
            "href",
            "/admin/invitations",
        );
    });

    it("offers no invitations on a public site, whatever the viewer holds", () => {
        // given
        const permissions: StaffPermission[] = ["view_admin_panel", "manage_user_account", "view_users"];

        // when
        renderConsole({ permissions, siteMode: "public" });

        // then
        expect(within(sections()).queryByRole("link", { name: "Invitations" })).not.toBeInTheDocument();
    });

    it("marks the page being viewed and only that page", () => {
        // when
        renderConsole({ route: "/admin/users" });

        // then
        expect(within(sections()).getByRole("link", { name: "Users" })).toHaveAttribute("aria-current", "page");
        expect(within(sections()).getByRole("link", { name: "Dashboard" })).not.toHaveAttribute("aria-current");
    });

    it("marks the dashboard on the console's front page", () => {
        // when
        renderConsole({ route: "/admin" });

        // then
        expect(within(sections()).getByRole("link", { name: "Dashboard" })).toHaveAttribute("aria-current", "page");
    });

    it("says which site and which mode the console is looking after", () => {
        // when
        renderConsole({ siteMode: "private" });

        // then
        const rail = screen.getByRole("complementary", { name: "Admin console" });
        expect(rail).toHaveTextContent("RetroTide");
        expect(rail).toHaveTextContent("Private site");
    });

    it("links back to the viewer's current organisation", () => {
        // given
        const organisations = [makeOrganisation({ slug: "harbour", name: "Harbour" })];

        // when
        renderConsole({ organisations });

        // then
        expect(within(screen.getByRole("banner")).getByRole("link", { name: "Back to app" })).toHaveAttribute(
            "href",
            "/o/harbour",
        );
    });

    it("links back to the account page when the viewer belongs to no organisation", () => {
        // when
        renderConsole({ organisations: [] });

        // then
        expect(within(screen.getByRole("banner")).getByRole("link", { name: "Back to app" })).toHaveAttribute(
            "href",
            "/account",
        );
    });

    it("keeps the phone menu closed until it is asked for", () => {
        // when
        renderConsole();

        // then
        expect(screen.getByRole("button", { name: "Admin menu" })).toHaveAttribute("aria-expanded", "false");
        expect(screen.queryByRole("button", { name: "Close menu" })).not.toBeInTheDocument();
    });

    it("opens the phone menu over the page", async () => {
        // given
        const user = userEvent.setup();
        renderConsole();

        // when
        await user.click(screen.getByRole("button", { name: "Admin menu" }));

        // then
        expect(screen.getByRole("button", { name: "Admin menu" })).toHaveAttribute("aria-expanded", "true");
        expect(screen.getByRole("button", { name: "Admin menu" })).toHaveAttribute(
            "aria-controls",
            screen.getByRole("complementary", { name: "Admin console" }).id,
        );
        expect(screen.getByRole("button", { name: "Close menu" })).toBeInTheDocument();
    });

    it("closes the phone menu once a page is picked", async () => {
        // given
        const user = userEvent.setup();
        renderConsole({ permissions: ["view_admin_panel", "view_users", "manage_settings"] });
        await user.click(screen.getByRole("button", { name: "Admin menu" }));

        // when
        await user.click(within(sections()).getByRole("link", { name: "Settings" }));

        // then
        expect(screen.getByRole("main")).toHaveTextContent("the settings page");
        expect(screen.getByRole("button", { name: "Admin menu" })).toHaveAttribute("aria-expanded", "false");
    });

    it("closes the phone menu with the escape key", async () => {
        // given
        const user = userEvent.setup();
        renderConsole();
        await user.click(screen.getByRole("button", { name: "Admin menu" }));

        // when
        await user.keyboard("{Escape}");

        // then
        expect(screen.getByRole("button", { name: "Admin menu" })).toHaveAttribute("aria-expanded", "false");
    });

    it("closes the phone menu when the page behind it is pressed", async () => {
        // given
        const user = userEvent.setup();
        renderConsole();
        await user.click(screen.getByRole("button", { name: "Admin menu" }));

        // when
        await user.click(screen.getByRole("button", { name: "Close menu" }));

        // then
        expect(screen.getByRole("button", { name: "Admin menu" })).toHaveAttribute("aria-expanded", "false");
        expect(screen.queryByRole("button", { name: "Close menu" })).not.toBeInTheDocument();
    });
});
