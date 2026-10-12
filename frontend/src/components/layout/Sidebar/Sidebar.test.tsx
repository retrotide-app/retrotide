import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeOrganisation, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { Sidebar } from "./Sidebar";

describe("Sidebar links for who is signed in", () => {
    it("shows a signed out visitor only the home page", () => {
        // given
        const visitor = null;

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, { user: visitor });

        // then
        expect(screen.getByRole("link", { name: "Home" })).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Account" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();
    });

    it("links a signed in user to their account", () => {
        // given
        const user = makeSessionUser();

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, { user });

        // then
        expect(screen.getByRole("link", { name: "Account" })).toHaveAttribute("href", "/account");
    });

    it("keeps the admin link from a user who is not staff", () => {
        // given
        const user = makeSessionUser();

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, { user, permissions: [] });

        // then
        expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();
    });

    it("links staff who may open the admin panel to it", () => {
        // given
        const options = { user: makeSessionUser(), permissions: ["view_admin_panel" as const] };

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, options);

        // then
        expect(screen.getByRole("link", { name: "Admin" })).toHaveAttribute("href", "/admin");
    });

    it("keeps the admin link from staff whose role lost the admin panel", () => {
        // given
        const options = { user: makeSessionUser(), permissions: ["view_users" as const] };

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, options);

        // then
        expect(screen.queryByRole("link", { name: "Admin" })).not.toBeInTheDocument();
    });

    it("marks Admin as the current section on any admin page", () => {
        // given
        const options = { user: makeSessionUser(), permissions: ["view_admin_panel" as const], route: "/admin/users" };

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, options);

        // then
        expect(screen.getByRole("link", { name: "Admin" })).toHaveAttribute("aria-current", "page");
        expect(screen.getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current");
    });
});

describe("Sidebar organisation", () => {
    const harbour = makeOrganisation({ slug: "harbour", name: "Harbour", role: "owner" });

    it("offers the organisation switcher to a signed in user", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations: [harbour] },
        });

        // then
        expect(screen.getByRole("combobox", { name: "Organisation" })).toHaveValue(harbour.id);
    });

    it("points Home, first in the list, at the current organisation's boards", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations: [harbour] },
        });

        // then
        const links = screen.getAllByRole("link").filter(link => link.closest("nav") !== null);
        expect(links[0]).toHaveAccessibleName("Home");
        expect(links[0]).toHaveAttribute("href", "/o/harbour");
    });

    it("marks Home as the current page on the organisation's boards, and only there", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations: [harbour] },
            route: "/o/harbour",
        });

        // then
        expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("aria-current", "page");
        expect(screen.getByRole("link", { name: "Teams" })).not.toHaveAttribute("aria-current");
    });

    it("leaves Home unmarked on the organisation's other sections", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations: [harbour] },
            route: "/o/harbour/teams",
        });

        // then
        expect(screen.getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current");
    });

    it("links the owner to every section of the current organisation", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations: [harbour] },
        });

        // then
        expect(screen.getByRole("link", { name: "Teams" })).toHaveAttribute("href", "/o/harbour/teams");
        expect(screen.getByRole("link", { name: "Action items" })).toHaveAttribute("href", "/o/harbour/action-items");
        expect(screen.getByRole("link", { name: "Members" })).toHaveAttribute("href", "/o/harbour/members");
        expect(screen.getByRole("link", { name: "Invites" })).toHaveAttribute("href", "/o/harbour/invites");
        expect(screen.getByRole("link", { name: "Organisation settings" })).toHaveAttribute(
            "href",
            "/o/harbour/settings",
        );
    });

    it("links a member to the teams and the action items only", () => {
        // given
        const organisations = [makeOrganisation({ slug: "harbour", role: "member" })];

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations },
        });

        // then
        expect(screen.getByRole("link", { name: "Teams" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Action items" })).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Members" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Invites" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Organisation settings" })).not.toBeInTheDocument();
    });

    it("marks Teams as the current section on a team's page", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: makeSessionUser(),
            org: { organisations: [harbour] },
            route: "/o/harbour/teams/t1",
        });

        // then
        expect(screen.getByRole("link", { name: "Teams" })).toHaveAttribute("aria-current", "page");
    });

    it("offers no switcher and no organisation links to a signed out visitor", () => {
        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, {
            user: null,
            org: { organisations: [harbour] },
        });

        // then
        expect(screen.queryByRole("combobox", { name: "Organisation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Teams" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", "/");
    });
});

describe("Sidebar brand", () => {
    it("names the brand link after the site and points it home", () => {
        // given
        const ui = <Sidebar open={false} onClose={vi.fn()} />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("link", { name: "RetroTide" })).toHaveAttribute("href", "/");
    });

    it("takes the brand name from the site information", () => {
        // given
        const siteInfo = { site_name: "Harbour Retros" };

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, { siteInfo });

        // then
        expect(screen.getByRole("link", { name: "Harbour Retros" })).toHaveAttribute("href", "/");
        expect(screen.queryByRole("link", { name: "RetroTide" })).not.toBeInTheDocument();
    });
});

describe("Sidebar navigation", () => {
    it("labels the sidebar and its primary navigation", () => {
        // given
        const ui = <Sidebar open={false} onClose={vi.fn()} />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("complementary", { name: "Main navigation" })).toHaveAttribute("id", "app-sidebar");
        expect(screen.getByRole("navigation", { name: "Primary" })).toBeInTheDocument();
    });

    it("marks Home as the current page at the root", () => {
        // given
        const route = "/";

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, { route });

        // then
        expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("aria-current", "page");
    });

    it("leaves Home unmarked on any other page", () => {
        // given
        const route = "/somewhere-else";

        // when
        renderWithProviders(<Sidebar open={false} onClose={vi.fn()} />, { route });

        // then
        expect(screen.getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current");
    });

    it("closes itself when a navigation link is followed", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Sidebar open onClose={onClose} />, { route: "/somewhere-else" });

        // when
        await user.click(screen.getByRole("link", { name: "Home" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("closes itself when the brand link is followed", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Sidebar open onClose={onClose} />, { route: "/somewhere-else" });

        // when
        await user.click(screen.getByRole("link", { name: "RetroTide" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });
});

describe("Sidebar shell", () => {
    it("leaves the page uncovered while the sidebar is closed", () => {
        // given
        const open = false;

        // when
        renderWithProviders(<Sidebar open={open} onClose={vi.fn()} />);

        // then
        expect(screen.queryByRole("button", { name: "Close menu" })).not.toBeInTheDocument();
    });

    it("covers the page with a close button while the sidebar is open", () => {
        // given
        const open = true;

        // when
        renderWithProviders(<Sidebar open={open} onClose={vi.fn()} />);

        // then
        expect(screen.getByRole("button", { name: "Close menu" })).toHaveAttribute("type", "button");
    });

    it("closes itself when the covering button is pressed", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Sidebar open onClose={onClose} />);

        // when
        await user.click(screen.getByRole("button", { name: "Close menu" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });
});
