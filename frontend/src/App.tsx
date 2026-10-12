import { type ReactNode, Suspense, useState } from "react";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router";
import { GuestClaimOffer } from "./components/GuestClaimOffer/GuestClaimOffer";
import { Header } from "./components/layout/Header/Header";
import { Sidebar } from "./components/layout/Sidebar/Sidebar";
import { LoadFailure } from "./components/LoadFailure/LoadFailure";
import { ProtectedRoute } from "./components/ProtectedRoute/ProtectedRoute";
import { RequireOrganisation } from "./components/RequireOrganisation/RequireOrganisation";
import { SessionCheckFailure } from "./components/SessionCheckFailure/SessionCheckFailure";
import { SessionExpiredBanner } from "./components/SessionExpiredBanner/SessionExpiredBanner";
import { JOIN_BOARD_ROUTE, SHARED_BOARD_ROUTE, SHARED_BOARD_SUMMARY_ROUTE } from "./domain/board/paths";
import { BOARD_PRINT_ROUTE } from "./domain/board/print";
import {
    COMPARE_EASYRETRO_PATH,
    LEGAL_ROUTE,
    offersMarketing,
    PRICING_PATH,
    TEMPLATE_ROUTE,
    TEMPLATES_PATH,
} from "./domain/marketing";
import { INVITE_PATH, ONBOARDING_PATH, organisationHomePath } from "./domain/organisations";
import { TWO_FACTOR_PATH } from "./domain/twoFactor";
import { useAuth } from "./hooks/useAuth";
import { useCurrentOrg } from "./hooks/useCurrentOrg";
import { useSiteInfo, useSiteInfoState } from "./hooks/useSiteInfo";
import {
    AcceptInvitationPage,
    AccountPage,
    AdminAuditLogPage,
    AdminDashboardPage,
    AdminInvitationsPage,
    AdminOrganisationPage,
    AdminOrganisationsPage,
    AdminPermissionsPage,
    AdminSettingsPage,
    AdminUserPage,
    AdminUsersPage,
    BoardPage,
    BoardPrintPage,
    BoardSettingsPage,
    BoardSummaryPage,
    CompareEasyRetroPage,
    ForgotPasswordPage,
    InviteAcceptancePage,
    JoinBoardPage,
    LandingPage,
    LegalPage,
    LoginPage,
    NewBoardPage,
    NotFoundPage,
    OnboardingPage,
    OrgActionItemsPage,
    OrgBoardsPage,
    OrgInvitesPage,
    OrgLayout,
    OrgMembersPage,
    OrgSettingsPage,
    OrgTeamsPage,
    PricingPage,
    RegisterPage,
    ResetPasswordPage,
    SavedTemplatesPage,
    SharedBoardPage,
    SharedBoardSummaryPage,
    StaffConsoleLayout,
    TeamPage,
    TemplatePage,
    TemplatesPage,
    TwoFactorPage,
    VerifyEmailPage,
} from "./pages/lazyPages";
import styles from "./App.module.css";

interface AppShellProps {
    children: ReactNode;
}

interface SiteUnavailableProps {
    message: string;
    onRetry: () => unknown;
}

function RouteFallback() {
    return <div className="loading">Loading...</div>;
}

function AppShell({ children }: AppShellProps) {
    const [sidebarOpen, setSidebarOpen] = useState(false);

    return (
        <div className={styles.layout}>
            <a href="#main-content" className="skip-link">
                Skip to content
            </a>
            <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
            <div className={styles.main}>
                <Header sidebarOpen={sidebarOpen} onToggleSidebar={() => setSidebarOpen(open => !open)} />
                <SessionExpiredBanner />
                <main id="main-content" tabIndex={-1} className={styles.content}>
                    <Suspense fallback={<RouteFallback />}>{children}</Suspense>
                </main>
            </div>
        </div>
    );
}

function CurrentOrganisationHome() {
    const { current } = useCurrentOrg();

    if (current === null) {
        throw new Error("no current organisation although the viewer belongs to one");
    }

    return <Navigate to={organisationHomePath(current.slug)} replace />;
}

function FrontDoor() {
    const { deployment_mode, site_mode } = useSiteInfo();
    const { user, loading, error, retry } = useAuth();

    if (user !== null) {
        return (
            <AppShell>
                <RequireOrganisation>
                    <CurrentOrganisationHome />
                </RequireOrganisation>
            </AppShell>
        );
    }

    if (loading) {
        return <RouteFallback />;
    }

    if (error !== null) {
        return (
            <AppShell>
                <SessionCheckFailure message={error.message} onRetry={retry} />
            </AppShell>
        );
    }

    if (!offersMarketing(deployment_mode, site_mode)) {
        return <Navigate to="/login" replace />;
    }

    return <LandingPage />;
}

function MarketingGate() {
    const { deployment_mode, site_mode } = useSiteInfo();

    if (!offersMarketing(deployment_mode, site_mode)) {
        return <Navigate to="/login" replace />;
    }

    return <Outlet />;
}

function AdminConsole() {
    const { user, permissions, loading } = useAuth();

    if (loading) {
        return <RouteFallback />;
    }

    if (user !== null && permissions.includes("view_admin_panel")) {
        return <StaffConsoleLayout />;
    }

    return (
        <AppShell>
            <ProtectedRoute permission="view_admin_panel" />
        </AppShell>
    );
}

function SiteAccessGate() {
    const { site_mode } = useSiteInfo();

    if (site_mode === "private") {
        return <ProtectedRoute />;
    }

    return <Outlet />;
}

function SiteUnavailable({ message, onRetry }: SiteUnavailableProps) {
    return (
        <main id="main-content" tabIndex={-1} className={styles.unavailable}>
            <div className={styles.unavailableCard}>
                <h1 className={styles.unavailableTitle}>This site could not load</h1>
                <LoadFailure message={message} onRetry={onRetry} />
            </div>
        </main>
    );
}

function AppRoutes() {
    return (
        <Suspense fallback={<RouteFallback />}>
            <Routes>
                <Route path="/" element={<FrontDoor />} />
                <Route element={<MarketingGate />}>
                    <Route path={PRICING_PATH} element={<PricingPage />} />
                    <Route path={COMPARE_EASYRETRO_PATH} element={<CompareEasyRetroPage />} />
                    <Route path={TEMPLATES_PATH} element={<TemplatesPage />} />
                    <Route path={TEMPLATE_ROUTE} element={<TemplatePage />} />
                    <Route path={LEGAL_ROUTE} element={<LegalPage />} />
                </Route>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/register" element={<RegisterPage />} />
                <Route path="/verify-email" element={<VerifyEmailPage />} />
                <Route path="/forgot-password" element={<ForgotPasswordPage />} />
                <Route path="/reset-password" element={<ResetPasswordPage />} />
                <Route path={TWO_FACTOR_PATH} element={<TwoFactorPage />} />
                <Route path="/accept-invitation" element={<AcceptInvitationPage />} />
                <Route path={INVITE_PATH} element={<InviteAcceptancePage />} />
                <Route path={JOIN_BOARD_ROUTE} element={<JoinBoardPage />} />
                <Route path={BOARD_PRINT_ROUTE} element={<BoardPrintPage />} />
                <Route
                    element={
                        <AppShell>
                            <Outlet />
                        </AppShell>
                    }
                >
                    <Route path={SHARED_BOARD_ROUTE} element={<SharedBoardPage />} />
                    <Route path={SHARED_BOARD_SUMMARY_ROUTE} element={<SharedBoardSummaryPage />} />
                </Route>
                <Route element={<SiteAccessGate />}>
                    <Route element={<ProtectedRoute />}>
                        <Route path={ONBOARDING_PATH} element={<OnboardingPage />} />
                    </Route>
                    <Route
                        element={
                            <AppShell>
                                <Outlet />
                            </AppShell>
                        }
                    >
                        <Route element={<ProtectedRoute />}>
                            <Route path="/account" element={<AccountPage />} />
                            <Route element={<RequireOrganisation />}>
                                <Route path="/o/:orgSlug" element={<OrgLayout />}>
                                    <Route index element={<OrgBoardsPage />} />
                                    <Route path="teams" element={<OrgTeamsPage />} />
                                    <Route path="teams/:teamId" element={<TeamPage />} />
                                    <Route path="action-items" element={<OrgActionItemsPage />} />
                                    <Route path="templates" element={<SavedTemplatesPage />} />
                                    <Route path="boards/new" element={<NewBoardPage />} />
                                    <Route path="boards/:boardId" element={<BoardPage />} />
                                    <Route path="boards/:boardId/settings" element={<BoardSettingsPage />} />
                                    <Route path="boards/:boardId/summary" element={<BoardSummaryPage />} />
                                    <Route path="members" element={<OrgMembersPage />} />
                                    <Route path="invites" element={<OrgInvitesPage />} />
                                    <Route path="settings" element={<OrgSettingsPage />} />
                                </Route>
                            </Route>
                        </Route>
                        <Route path="*" element={<NotFoundPage />} />
                    </Route>
                    <Route path="/admin" element={<AdminConsole />}>
                        <Route index element={<AdminDashboardPage />} />
                        <Route
                            path="users"
                            element={
                                <ProtectedRoute permission="view_users" headingLevel={2}>
                                    <AdminUsersPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="users/:userId"
                            element={
                                <ProtectedRoute permission="view_users" headingLevel={2}>
                                    <AdminUserPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="organisations"
                            element={
                                <ProtectedRoute permission="view_organisations" headingLevel={2}>
                                    <AdminOrganisationsPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="organisations/:orgId"
                            element={
                                <ProtectedRoute permission="view_organisations" headingLevel={2}>
                                    <AdminOrganisationPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="invitations"
                            element={
                                <ProtectedRoute permission="manage_user_account" headingLevel={2}>
                                    <AdminInvitationsPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="permissions"
                            element={
                                <ProtectedRoute permission="manage_roles" headingLevel={2}>
                                    <AdminPermissionsPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="settings"
                            element={
                                <ProtectedRoute permission="manage_settings" headingLevel={2}>
                                    <AdminSettingsPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route
                            path="audit-log"
                            element={
                                <ProtectedRoute permission="view_audit_log" headingLevel={2}>
                                    <AdminAuditLogPage />
                                </ProtectedRoute>
                            }
                        />
                        <Route path="*" element={<NotFoundPage />} />
                    </Route>
                </Route>
            </Routes>
            <GuestClaimOffer />
        </Suspense>
    );
}

function SiteGate() {
    const { siteInfo, error, retry } = useSiteInfoState();

    if (siteInfo === null && error !== null) {
        return <SiteUnavailable message={error.message} onRetry={retry} />;
    }

    if (siteInfo === null) {
        return <RouteFallback />;
    }

    return <AppRoutes />;
}

export default function App() {
    return (
        <BrowserRouter>
            <SiteGate />
        </BrowserRouter>
    );
}
