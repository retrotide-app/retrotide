import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren } from "react";
import { describe, expect, it, vi } from "vitest";
import App from "./App";
import { PENDING_INVITE_STORAGE_KEY } from "./platform/pendingInvite";
import { makeApiError, makeOrganisation, makeSessionUser } from "./test-utils/fixtures";
import { renderWithProviders } from "./test-utils/render";

vi.mock("react-router", async importOriginal => {
    const actual = await importOriginal<typeof import("react-router")>();

    return { ...actual, BrowserRouter: ({ children }: PropsWithChildren) => <>{children}</> };
});

vi.mock("./pages/lazyPages", async () => {
    const { Outlet } = await import("react-router");

    return {
        LandingPage: () => <h1>Landing page</h1>,
        NotFoundPage: () => <h1>Not found page</h1>,
        PricingPage: () => <h1>Pricing page</h1>,
        CompareEasyRetroPage: () => <h1>Compare EasyRetro page</h1>,
        SelfHostPage: () => <h1>Self-host page</h1>,
        TemplatesPage: () => <h1>Templates page</h1>,
        TemplatePage: () => <h1>Template page</h1>,
        LegalPage: () => <h1>Legal page</h1>,
        LoginPage: () => <h1>Login page</h1>,
        RegisterPage: () => <h1>Register page</h1>,
        VerifyEmailPage: () => <h1>Verify email page</h1>,
        ForgotPasswordPage: () => <h1>Forgot password page</h1>,
        ResetPasswordPage: () => <h1>Reset password page</h1>,
        TwoFactorPage: () => <h1>Two-factor page</h1>,
        AcceptInvitationPage: () => <h1>Accept invitation page</h1>,
        InviteAcceptancePage: () => <h1>Invite acceptance page</h1>,
        OnboardingPage: () => <h1>Onboarding page</h1>,
        AccountPage: () => <h1>Account page</h1>,
        OrgLayout: () => <Outlet />,
        OrgBoardsPage: () => <h1>Organisation boards page</h1>,
        OrgTeamsPage: () => <h1>Organisation teams page</h1>,
        OrgActionItemsPage: () => <h1>Organisation action items page</h1>,
        SavedTemplatesPage: () => <h1>Saved templates page</h1>,
        TeamPage: () => <h1>Team page</h1>,
        BoardPage: () => <h1>Board page</h1>,
        SharedBoardPage: () => <h1>Shared board page</h1>,
        SharedBoardSummaryPage: () => <h1>Shared board summary page</h1>,
        BoardPrintPage: () => <h1>Board print page</h1>,
        JoinBoardPage: () => <h1>Join board page</h1>,
        BoardSettingsPage: () => <h1>Board settings page</h1>,
        BoardSummaryPage: () => <h1>Board summary page</h1>,
        NewBoardPage: () => <h1>New board page</h1>,
        OrgMembersPage: () => <h1>Organisation members page</h1>,
        OrgInvitesPage: () => <h1>Organisation invites page</h1>,
        OrgSettingsPage: () => <h1>Organisation settings page</h1>,
        StaffConsoleLayout: () => (
            <div>
                <h1>Admin console</h1>
                <Outlet />
            </div>
        ),
        AdminDashboardPage: () => <h2>Admin dashboard page</h2>,
        AdminUsersPage: () => <h2>Admin users page</h2>,
        AdminUserPage: () => <h2>Admin user page</h2>,
        AdminOrganisationsPage: () => <h2>Admin organisations page</h2>,
        AdminOrganisationPage: () => <h2>Admin organisation page</h2>,
        AdminInvitationsPage: () => <h2>Admin invitations page</h2>,
        AdminPermissionsPage: () => <h2>Admin permissions page</h2>,
        AdminSettingsPage: () => <h2>Admin settings page</h2>,
        AdminAuditLogPage: () => <h2>Admin audit log page</h2>,
    };
});

vi.mock("./components/GuestClaimOffer/GuestClaimOffer", () => ({
    GuestClaimOffer: () => <p>Guest claim offer</p>,
}));

const selfHosted = { deployment_mode: "self_hosted" } as const;

const privateSite = { site_mode: "private", registration_open: false } as const;

const harbour = makeOrganisation({ slug: "harbour", name: "Harbour" });

const member = { organisations: [harbour] };

const signedIn = { user: makeSessionUser(), org: member };

describe("App", () => {
    it("shows the landing page at the root of the hosted site", () => {
        // given
        const siteInfo = { deployment_mode: "hosted" } as const;

        // when
        renderWithProviders(<App />, { route: "/", siteInfo });

        // then
        expect(screen.getByRole("heading", { name: "Landing page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation boards page" })).not.toBeInTheDocument();
    });

    it("draws the landing page without the app's header and sidebar", () => {
        // given
        const siteInfo = { deployment_mode: "hosted" } as const;

        // when
        renderWithProviders(<App />, { route: "/", siteInfo });

        // then
        expect(screen.queryByRole("button", { name: "Toggle menu" })).not.toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
    });

    it("sends a signed out visitor of a self-hosted instance from the root to sign in, because boards need an account", () => {
        // given
        const siteInfo = selfHosted;

        // when
        renderWithProviders(<App />, { route: "/", siteInfo });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
    });

    it("opens the current organisation's boards at the root of a self-hosted instance once someone is signed in", () => {
        // given
        const siteInfo = selfHosted;

        // when
        renderWithProviders(<App />, { route: "/", siteInfo, ...signedIn });

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "Organisation boards page" }),
        );
    });

    it("opens the organisation the member chose last, not the first one listed", () => {
        // given
        const lighthouse = makeOrganisation({ id: "o2", slug: "lighthouse", name: "Lighthouse" });
        const org = { organisations: [harbour, lighthouse], current: lighthouse };

        // when
        renderWithProviders(<App />, { route: "/", user: makeSessionUser(), org });

        // then
        expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", "/o/lighthouse");
        expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("aria-current", "page");
    });

    it("shows the app home page at the root of the hosted site once someone is signed in", () => {
        // given
        const user = makeSessionUser();

        // when
        renderWithProviders(<App />, { route: "/", user, org: member });

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "Organisation boards page" }),
        );
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
    });

    it("waits for the session before choosing between the landing page and the app on the hosted site", () => {
        // given
        const auth = { loading: true };

        // when
        renderWithProviders(<App />, { route: "/", auth });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation boards page" })).not.toBeInTheDocument();
    });

    it("says the sign-in could not be checked, instead of the landing page, when the session check fails", () => {
        // given
        const auth = { error: makeApiError({ status: 500, message: "failed to load the session" }) };

        // when
        renderWithProviders(<App />, { route: "/", auth });

        // then
        expect(screen.getByRole("heading", { name: "We could not check your sign-in" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("failed to load the session");
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
    });

    it("checks the session again when the visitor tries again from the front door", async () => {
        // given
        const retry = vi.fn().mockResolvedValue(undefined);
        const auth = { error: makeApiError({ status: 500, message: "failed to load the session" }), retry };
        const user = userEvent.setup();
        renderWithProviders(<App />, { route: "/", auth });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(retry).toHaveBeenCalledOnce();
    });

    it("shows the not found page for a route nobody claims on the hosted site", () => {
        // given
        const route = "/no/such/page";

        // when
        renderWithProviders(<App />, { route });

        // then
        expect(screen.getByRole("heading", { name: "Not found page" })).toBeInTheDocument();
        expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Not found page" }));
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
    });

    it("shows the not found page for a route nobody claims on a self-hosted instance", () => {
        // given
        const route = "/no/such/page";

        // when
        renderWithProviders(<App />, { route, siteInfo: selfHosted });

        // then
        expect(screen.getByRole("heading", { name: "Not found page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation boards page" })).not.toBeInTheDocument();
    });

    it("draws the page inside the main landmark, beside the header and the sidebar", () => {
        // given
        const route = "/";

        // when
        renderWithProviders(<App />, { route, siteInfo: selfHosted, ...signedIn });

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "Organisation boards page" }),
        );
        expect(screen.getByRole("banner")).toBeInTheDocument();
        expect(screen.getByRole("complementary", { name: "Main navigation" })).toBeInTheDocument();
    });

    it("starts with the sidebar closed", () => {
        // given
        const route = "/";

        // when
        renderWithProviders(<App />, { route, siteInfo: selfHosted, ...signedIn });

        // then
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-expanded", "false");
        expect(screen.queryByRole("button", { name: "Close menu" })).not.toBeInTheDocument();
    });

    it("opens the sidebar from the menu button", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<App />, { route: "/", siteInfo: selfHosted, ...signedIn });

        // when
        await user.click(screen.getByRole("button", { name: "Toggle menu" }));

        // then
        expect(screen.getByRole("button", { name: "Close menu" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-expanded", "true");
    });

    it("closes the sidebar again from its overlay", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<App />, { route: "/", siteInfo: selfHosted, ...signedIn });
        await user.click(screen.getByRole("button", { name: "Toggle menu" }));

        // when
        await user.click(screen.getByRole("button", { name: "Close menu" }));

        // then
        expect(screen.queryByRole("button", { name: "Close menu" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-expanded", "false");
    });

    it("closes the sidebar when the menu button is pressed a second time", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<App />, { route: "/", siteInfo: selfHosted, ...signedIn });
        await user.click(screen.getByRole("button", { name: "Toggle menu" }));

        // when
        await user.click(screen.getByRole("button", { name: "Toggle menu" }));

        // then
        expect(screen.queryByRole("button", { name: "Close menu" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-expanded", "false");
    });

    it("points the skip link at the main landmark, which can take its focus", () => {
        // given
        const route = "/";

        // when
        renderWithProviders(<App />, { route, siteInfo: selfHosted, ...signedIn });

        // then
        expect(screen.getByRole("link", { name: "Skip to content" })).toHaveAttribute("href", "#main-content");
        expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
        expect(screen.getByRole("main")).toHaveAttribute("tabindex", "-1");
    });

    it("puts the skip link ahead of everything else a keyboard reaches", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<App />, { route: "/", siteInfo: selfHosted, ...signedIn });

        // when
        await user.tab();

        // then
        expect(screen.getByRole("link", { name: "Skip to content" })).toHaveFocus();
    });

    it("keeps the session expired banner away while nobody is signed in", () => {
        // given
        const visitor = null;

        // when
        renderWithProviders(<App />, { route: "/no/such/page", user: visitor, siteInfo: selfHosted });

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("draws no verification banner, because an account signs in only once its address is verified", () => {
        // given
        const user = makeSessionUser({ email_verified: false });

        // when
        renderWithProviders(<App />, { route: "/", user, org: member });

        // then
        expect(screen.getByRole("heading", { name: "Organisation boards page" })).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Email verification" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Resend link" })).not.toBeInTheDocument();
    });
});

describe("App while the site info is not in", () => {
    it("shows a loading placeholder and no page until the site info arrives", () => {
        // given
        const siteInfoContext = { siteInfo: null, loading: true };

        // when
        renderWithProviders(<App />, { route: "/", siteInfoContext });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
    });

    it("says the site could not load, with the server's reason, when the site info fails", () => {
        // given
        const error = makeApiError({ status: 500, message: "failed to read the site info" });

        // when
        renderWithProviders(<App />, { route: "/login", siteInfoContext: { siteInfo: null, error } });

        // then
        expect(screen.getByRole("heading", { name: "This site could not load" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("failed to read the site info");
        expect(screen.queryByRole("heading", { name: "Login page" })).not.toBeInTheDocument();
    });

    it("asks for the site info again when the visitor tries again", async () => {
        // given
        const retry = vi.fn().mockResolvedValue(undefined);
        const error = makeApiError({ status: 500, message: "failed to read the site info" });
        const user = userEvent.setup();
        renderWithProviders(<App />, { route: "/", siteInfoContext: { siteInfo: null, error, retry } });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(retry).toHaveBeenCalledOnce();
    });
});

describe("App auth routes", () => {
    const pages = [
        { route: "/login", heading: "Login page" },
        { route: "/register", heading: "Register page" },
        { route: "/verify-email", heading: "Verify email page" },
        { route: "/forgot-password", heading: "Forgot password page" },
        { route: "/reset-password", heading: "Reset password page" },
        { route: "/two-factor", heading: "Two-factor page" },
        { route: "/accept-invitation", heading: "Accept invitation page" },
        { route: "/invite", heading: "Invite acceptance page" },
    ];

    it.each(pages)("draws $route on its own, without the app's chrome", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route });

        // then
        expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
        expect(screen.queryByRole("banner")).not.toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
    });

    it("resolves the auth routes on a self-hosted instance too", () => {
        // when
        renderWithProviders(<App />, { route: "/register", siteInfo: selfHosted });

        // then
        expect(screen.getByRole("heading", { name: "Register page" })).toBeInTheDocument();
    });

    it("keeps the reset token in the fragment reachable, because the route ignores it", () => {
        // when
        renderWithProviders(<App />, { route: "/reset-password#9f2c0a" });

        // then
        expect(screen.getByRole("heading", { name: "Reset password page" })).toBeInTheDocument();
    });

    it("keeps the invitation token in the fragment reachable, because the route ignores it", () => {
        // when
        renderWithProviders(<App />, { route: "/accept-invitation#9f2c0a" });

        // then
        expect(screen.getByRole("heading", { name: "Accept invitation page" })).toBeInTheDocument();
    });

    it("keeps an organisation invite's token in the fragment reachable, because the route ignores it", () => {
        // when
        renderWithProviders(<App />, { route: "/invite#9f2c0a" });

        // then
        expect(screen.getByRole("heading", { name: "Invite acceptance page" })).toBeInTheDocument();
    });

    it("opens an organisation invite for a signed in user too, whatever organisations they hold", () => {
        // when
        renderWithProviders(<App />, { route: "/invite", user: makeSessionUser(), org: member });

        // then
        expect(screen.getByRole("heading", { name: "Invite acceptance page" })).toBeInTheDocument();
    });
});

describe("App on a private site", () => {
    it("sends a signed out visitor from the root to sign in, with no landing page", () => {
        // when
        renderWithProviders(<App />, { route: "/", siteInfo: privateSite });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Landing page" })).not.toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation boards page" })).not.toBeInTheDocument();
    });

    it("sends a signed out visitor of a private self-hosted instance to sign in too", () => {
        // when
        renderWithProviders(<App />, { route: "/", siteInfo: { ...selfHosted, ...privateSite } });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation boards page" })).not.toBeInTheDocument();
    });

    it("shows a signed in user the app home page", () => {
        // when
        renderWithProviders(<App />, { route: "/", siteInfo: privateSite, user: makeSessionUser(), org: member });

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "Organisation boards page" }),
        );
    });

    it("waits for the session before sending anyone anywhere", () => {
        // when
        renderWithProviders(<App />, { route: "/no/such/page", siteInfo: privateSite, auth: { loading: true } });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Login page" })).not.toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Not found page" })).not.toBeInTheDocument();
    });

    it("says the sign-in could not be checked, instead of sending the visitor away, when the session check fails", () => {
        // given
        const auth = { error: makeApiError({ status: 500, message: "failed to load the session" }) };

        // when
        renderWithProviders(<App />, { route: "/no/such/page", siteInfo: privateSite, auth });

        // then
        expect(screen.getByRole("heading", { name: "We could not check your sign-in" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Login page" })).not.toBeInTheDocument();
    });

    it("sends a signed out visitor from any other page to sign in", () => {
        // when
        renderWithProviders(<App />, { route: "/no/such/page", siteInfo: privateSite });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Not found page" })).not.toBeInTheDocument();
        expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    });

    it("shows a signed in user the not found page for a route nobody claims", () => {
        // when
        renderWithProviders(<App />, { route: "/no/such/page", siteInfo: privateSite, user: makeSessionUser() });

        // then
        expect(screen.getByRole("heading", { name: "Not found page" })).toBeInTheDocument();
    });

    const openPages = [
        { route: "/login", heading: "Login page" },
        { route: "/register", heading: "Register page" },
        { route: "/verify-email", heading: "Verify email page" },
        { route: "/forgot-password", heading: "Forgot password page" },
        { route: "/reset-password", heading: "Reset password page" },
        { route: "/two-factor", heading: "Two-factor page" },
        { route: "/accept-invitation", heading: "Accept invitation page" },
        { route: "/invite", heading: "Invite acceptance page" },
        { route: "/b/k7Q2", heading: "Join board page" },
        { route: "/boards/00000000-0000-4000-8000-0000000000b1", heading: "Shared board page" },
    ];

    it.each(openPages)("keeps $route open to a signed out visitor", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, siteInfo: privateSite });

        // then
        expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
    });

    it("lets a guest reach the board they joined, as the server lets their guest pass through", () => {
        // when
        renderWithProviders(<App />, {
            route: "/boards/00000000-0000-4000-8000-0000000000b1",
            siteInfo: privateSite,
            auth: { guestId: "g1" },
        });

        // then
        expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Shared board page" }));
        expect(screen.queryByRole("heading", { name: "Login page" })).not.toBeInTheDocument();
    });
});

describe("App share link routes", () => {
    it("draws the join page on its own, without the app's header and sidebar", () => {
        // when
        renderWithProviders(<App />, { route: "/b/k7Q2" });

        // then
        expect(screen.getByRole("heading", { name: "Join board page" })).toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
    });

    it("draws a joined board inside the app shell", () => {
        // when
        renderWithProviders(<App />, { route: "/boards/00000000-0000-4000-8000-0000000000b1" });

        // then
        expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Shared board page" }));
        expect(screen.getByRole("complementary", { name: "Main navigation" })).toBeInTheDocument();
    });

    it("draws a joined board's summary inside the app shell", () => {
        // when
        renderWithProviders(<App />, { route: "/boards/00000000-0000-4000-8000-0000000000b1/summary" });

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "Shared board summary page" }),
        );
    });

    it("draws a board's print page on its own, without the app's header and sidebar", () => {
        // when
        renderWithProviders(<App />, { route: "/boards/00000000-0000-4000-8000-0000000000b1/print?then=print" });

        // then
        expect(screen.getByRole("heading", { name: "Board print page" })).toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
    });

    it("keeps the guest claim offer beside every route, so it follows any sign-in", () => {
        // when
        renderWithProviders(<App />, { route: "/", user: makeSessionUser(), org: member });

        // then
        expect(screen.getByText("Guest claim offer")).toBeInTheDocument();
    });
});

describe("App for a signed in user with no organisation", () => {
    it("sends them from the root to onboarding", () => {
        // when
        renderWithProviders(<App />, { route: "/", user: makeSessionUser(), org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Onboarding page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation boards page" })).not.toBeInTheDocument();
    });

    it("sends them to the invite they are part way through instead, so they get no workspace of their own", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        renderWithProviders(<App />, { route: "/", user: makeSessionUser(), org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Invite acceptance page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding page" })).not.toBeInTheDocument();
    });

    it("sends them from an organisation page to onboarding", () => {
        // when
        renderWithProviders(<App />, {
            route: "/o/harbour/teams",
            user: makeSessionUser(),
            org: { organisations: [] },
        });

        // then
        expect(screen.getByRole("heading", { name: "Onboarding page" })).toBeInTheDocument();
    });

    it("keeps their account settings open, so they can still set a password or delete the account", () => {
        // when
        renderWithProviders(<App />, { route: "/account", user: makeSessionUser(), org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Account page" })).toBeInTheDocument();
    });

    it("waits for their organisations before sending them anywhere", () => {
        // when
        renderWithProviders(<App />, {
            route: "/",
            user: makeSessionUser(),
            org: { organisations: [], loading: true },
        });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding page" })).not.toBeInTheDocument();
    });

    it("shows why their organisations could not be read, instead of sending them to onboarding", () => {
        // given
        const error = makeApiError({ status: 500, message: "failed to list your organisations" });

        // when
        renderWithProviders(<App />, { route: "/", user: makeSessionUser(), org: { organisations: [], error } });

        // then
        expect(screen.getByRole("heading", { name: "We could not load your organisations" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding page" })).not.toBeInTheDocument();
    });
});

describe("App onboarding route", () => {
    it("draws onboarding on its own, without the app's chrome", () => {
        // when
        renderWithProviders(<App />, { route: "/onboarding", user: makeSessionUser(), org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Onboarding page" })).toBeInTheDocument();
        expect(screen.queryByRole("banner")).not.toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
    });

    it("sends a signed out visitor to sign in", () => {
        // when
        renderWithProviders(<App />, { route: "/onboarding" });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding page" })).not.toBeInTheDocument();
    });

    it("sends a signed out visitor of a private site to sign in", () => {
        // when
        renderWithProviders(<App />, { route: "/onboarding", siteInfo: privateSite });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
    });
});

describe("App organisation routes", () => {
    const sections = [
        { route: "/o/harbour/teams", heading: "Organisation teams page" },
        { route: "/o/harbour/teams/t1", heading: "Team page" },
        { route: "/o/harbour/action-items", heading: "Organisation action items page" },
        { route: "/o/harbour/boards/b1", heading: "Board page" },
        { route: "/o/harbour/boards/b1/summary", heading: "Board summary page" },
        { route: "/o/harbour/boards/new", heading: "New board page" },
        { route: "/o/harbour/boards/new?team=t1", heading: "New board page" },
        { route: "/o/harbour/members", heading: "Organisation members page" },
        { route: "/o/harbour/invites", heading: "Organisation invites page" },
        { route: "/o/harbour/settings", heading: "Organisation settings page" },
    ];

    it.each(sections)("draws $route inside the app for a member", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, user: makeSessionUser(), org: member });

        // then
        expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: heading }));
        expect(screen.getByRole("complementary", { name: "Main navigation" })).toBeInTheDocument();
    });

    it("opens an organisation's boards, its home, from its bare address", () => {
        // when
        renderWithProviders(<App />, { route: "/o/harbour", user: makeSessionUser(), org: member });

        // then
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "Organisation boards page" }),
        );
        expect(screen.queryByRole("heading", { name: "Organisation teams page" })).not.toBeInTheDocument();
    });

    it("sends a signed out visitor to sign in", () => {
        // when
        renderWithProviders(<App />, { route: "/o/harbour/members" });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation members page" })).not.toBeInTheDocument();
    });
});

describe("App account route", () => {
    it("sends a signed out visitor to sign in", () => {
        // when
        renderWithProviders(<App />, { route: "/account" });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Account page" })).not.toBeInTheDocument();
    });

    it("shows a signed in user their account inside the app", () => {
        // when
        renderWithProviders(<App />, { route: "/account", user: makeSessionUser() });

        // then
        expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Account page" }));
    });
});

describe("App admin routes", () => {
    it("sends a signed out visitor to sign in", () => {
        // when
        renderWithProviders(<App />, { route: "/admin/users" });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
    });

    it("waits for the session without drawing the member app's chrome", () => {
        // when
        renderWithProviders(<App />, { route: "/admin/users", auth: { loading: true } });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Admin console" })).not.toBeInTheDocument();
    });

    it("refuses a user who is not staff", () => {
        // when
        renderWithProviders(<App />, { route: "/admin", user: makeSessionUser(), permissions: [] });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "You cannot open this page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Admin console" })).not.toBeInTheDocument();
    });

    it("refuses a user who is not staff inside the member app, not the console", () => {
        // when
        renderWithProviders(<App />, { route: "/admin", user: makeSessionUser(), permissions: [] });

        // then
        expect(screen.getByRole("complementary", { name: "Main navigation" })).toBeInTheDocument();
        expect(screen.getByRole("main")).toContainElement(
            screen.getByRole("heading", { name: "You cannot open this page" }),
        );
    });

    it("refuses staff whose role cannot open the admin panel, whatever else it holds", () => {
        // when
        renderWithProviders(<App />, { route: "/admin/users", user: makeSessionUser(), permissions: ["view_users"] });

        // then
        expect(screen.getByRole("heading", { name: "You cannot open this page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Admin users page" })).not.toBeInTheDocument();
    });

    it("opens the dashboard inside the console for staff", () => {
        // when
        renderWithProviders(<App />, {
            route: "/admin",
            user: makeSessionUser(),
            permissions: ["view_admin_panel"],
        });

        // then
        expect(screen.getByRole("heading", { name: "Admin console" })).toBeInTheDocument();
        expect(screen.getByRole("heading", { name: "Admin dashboard page" })).toBeInTheDocument();
    });

    it("draws the console without the member app's sidebar", () => {
        // when
        renderWithProviders(<App />, {
            route: "/admin/users",
            user: makeSessionUser(),
            permissions: ["view_admin_panel", "view_users"],
        });

        // then
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Toggle menu" })).not.toBeInTheDocument();
    });

    it("sends staff who belong to no organisation from the front door to the console", () => {
        // when
        renderWithProviders(<App />, {
            route: "/",
            user: makeSessionUser(),
            permissions: ["view_admin_panel"],
            org: { organisations: [] },
        });

        // then
        expect(screen.getByRole("heading", { name: "Admin dashboard page" })).toBeInTheDocument();
    });

    it("shows a page not found inside the console for an unknown admin address", () => {
        // when
        renderWithProviders(<App />, {
            route: "/admin/nowhere",
            user: makeSessionUser(),
            permissions: ["view_admin_panel"],
        });

        // then
        expect(screen.getByRole("heading", { name: "Admin console" })).toBeInTheDocument();
        expect(screen.getByRole("heading", { name: "Not found page" })).toBeInTheDocument();
    });

    const sections = [
        { route: "/admin/users", permission: "view_users" as const, heading: "Admin users page" },
        { route: "/admin/users/u1", permission: "view_users" as const, heading: "Admin user page" },
        {
            route: "/admin/organisations",
            permission: "view_organisations" as const,
            heading: "Admin organisations page",
        },
        {
            route: "/admin/organisations/o1",
            permission: "view_organisations" as const,
            heading: "Admin organisation page",
        },
        {
            route: "/admin/invitations",
            permission: "manage_user_account" as const,
            heading: "Admin invitations page",
        },
        { route: "/admin/permissions", permission: "manage_roles" as const, heading: "Admin permissions page" },
        { route: "/admin/settings", permission: "manage_settings" as const, heading: "Admin settings page" },
        { route: "/admin/audit-log", permission: "view_audit_log" as const, heading: "Admin audit log page" },
    ];

    it.each(sections)("opens $route for staff holding $permission", ({ route, permission, heading }) => {
        // when
        renderWithProviders(<App />, {
            route,
            user: makeSessionUser(),
            permissions: ["view_admin_panel", permission],
        });

        // then
        expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
    });

    it.each(sections)("refuses $route to staff without $permission", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, user: makeSessionUser(), permissions: ["view_admin_panel"] });

        // then
        expect(screen.getByRole("heading", { level: 2, name: "You cannot open this page" })).toBeInTheDocument();
        expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
        expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    });
});

describe("App marketing routes", () => {
    it("hides the pricing page while pricing is hidden", () => {
        // when
        renderWithProviders(<App />, { route: "/pricing", siteInfo: { deployment_mode: "hosted" } });

        // then
        expect(screen.getByRole("heading", { name: "Not found page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Pricing page" })).not.toBeInTheDocument();
    });

    const pages = [
        { route: "/compare/easyretro", heading: "Compare EasyRetro page" },
        { route: "/self-host", heading: "Self-host page" },
        { route: "/templates", heading: "Templates page" },
        { route: "/templates/sailboat", heading: "Template page" },
        { route: "/legal/privacy", heading: "Legal page" },
        { route: "/legal/sub-processors", heading: "Legal page" },
    ];

    it.each(pages)("shows $route to a signed out visitor of the hosted site", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, siteInfo: { deployment_mode: "hosted" } });

        // then
        expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
        expect(screen.queryByRole("complementary", { name: "Main navigation" })).not.toBeInTheDocument();
    });

    it.each(pages)("shows $route to a signed in member of the hosted site too", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, siteInfo: { deployment_mode: "hosted" }, ...signedIn });

        // then
        expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
    });

    it.each(pages)("sends a visitor of a self-hosted instance from $route to sign in", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, siteInfo: selfHosted });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    });

    it.each(pages)("sends a visitor of a private site from $route to sign in", ({ route, heading }) => {
        // when
        renderWithProviders(<App />, { route, siteInfo: privateSite });

        // then
        expect(screen.getByRole("heading", { name: "Login page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    });
});
