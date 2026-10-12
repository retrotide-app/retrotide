import { type ComponentType, lazy } from "react";

function named<K extends string>(loader: () => Promise<Record<K, ComponentType>>, name: K) {
    return lazy(() => loader().then(module => ({ default: module[name] })));
}

export const LandingPage = named(() => import("./landing/LandingPage"), "LandingPage");
export const NotFoundPage = named(() => import("./notfound/NotFoundPage"), "NotFoundPage");
export const PricingPage = named(() => import("./pricing/PricingPage"), "PricingPage");
export const CompareEasyRetroPage = named(() => import("./compare/CompareEasyRetroPage"), "CompareEasyRetroPage");
export const TemplatesPage = named(() => import("./templates/TemplatesPage"), "TemplatesPage");
export const TemplatePage = named(() => import("./templates/TemplatePage"), "TemplatePage");
export const LegalPage = named(() => import("./legal/LegalPage"), "LegalPage");

export const LoginPage = named(() => import("./auth/login/LoginPage"), "LoginPage");
export const RegisterPage = named(() => import("./auth/register/RegisterPage"), "RegisterPage");
export const VerifyEmailPage = named(() => import("./auth/verifyemail/VerifyEmailPage"), "VerifyEmailPage");
export const ForgotPasswordPage = named(() => import("./auth/forgotpassword/ForgotPasswordPage"), "ForgotPasswordPage");
export const ResetPasswordPage = named(() => import("./auth/resetpassword/ResetPasswordPage"), "ResetPasswordPage");
export const TwoFactorPage = named(() => import("./auth/twofactor/TwoFactorPage"), "TwoFactorPage");
export const AcceptInvitationPage = named(
    () => import("./auth/acceptinvitation/AcceptInvitationPage"),
    "AcceptInvitationPage",
);

export const InviteAcceptancePage = named(() => import("./invite/InviteAcceptancePage"), "InviteAcceptancePage");
export const OnboardingPage = named(() => import("./onboarding/OnboardingPage"), "OnboardingPage");

export const AccountPage = named(() => import("./account/AccountPage"), "AccountPage");

export const OrgLayout = named(() => import("./org/layout/OrgLayout"), "OrgLayout");
export const OrgBoardsPage = named(() => import("./org/boards/OrgBoardsPage"), "OrgBoardsPage");
export const OrgTeamsPage = named(() => import("./org/teams/OrgTeamsPage"), "OrgTeamsPage");
export const OrgActionItemsPage = named(() => import("./org/actionitems/OrgActionItemsPage"), "OrgActionItemsPage");
export const SavedTemplatesPage = named(() => import("./org/templates/SavedTemplatesPage"), "SavedTemplatesPage");
export const OrgMembersPage = named(() => import("./org/members/OrgMembersPage"), "OrgMembersPage");
export const OrgInvitesPage = named(() => import("./org/invites/OrgInvitesPage"), "OrgInvitesPage");
export const OrgSettingsPage = named(() => import("./org/settings/OrgSettingsPage"), "OrgSettingsPage");
export const TeamPage = named(() => import("./team/TeamPage"), "TeamPage");
export const BoardPage = named(() => import("./board/BoardPage"), "BoardPage");
export const SharedBoardPage = named(() => import("./board/SharedBoardPage"), "SharedBoardPage");
export const JoinBoardPage = named(() => import("./join/JoinBoardPage"), "JoinBoardPage");
export const BoardSettingsPage = named(() => import("./board/BoardSettingsPage"), "BoardSettingsPage");
export const BoardSummaryPage = named(() => import("./board/BoardSummaryPage"), "BoardSummaryPage");
export const SharedBoardSummaryPage = named(() => import("./board/SharedBoardSummaryPage"), "SharedBoardSummaryPage");
export const BoardPrintPage = named(() => import("./board/BoardPrintPage"), "BoardPrintPage");
export const NewBoardPage = named(() => import("./newboard/NewBoardPage"), "NewBoardPage");

export const StaffConsoleLayout = named(() => import("./admin/layout/StaffConsoleLayout"), "StaffConsoleLayout");
export const AdminDashboardPage = named(() => import("./admin/dashboard/AdminDashboardPage"), "AdminDashboardPage");
export const AdminUsersPage = named(() => import("./admin/users/AdminUsersPage"), "AdminUsersPage");
export const AdminUserPage = named(() => import("./admin/user/AdminUserPage"), "AdminUserPage");
export const AdminOrganisationsPage = named(
    () => import("./admin/organisations/AdminOrganisationsPage"),
    "AdminOrganisationsPage",
);
export const AdminOrganisationPage = named(
    () => import("./admin/organisations/AdminOrganisationPage"),
    "AdminOrganisationPage",
);
export const AdminInvitationsPage = named(
    () => import("./admin/invitations/AdminInvitationsPage"),
    "AdminInvitationsPage",
);
export const AdminPermissionsPage = named(
    () => import("./admin/permissions/AdminPermissionsPage"),
    "AdminPermissionsPage",
);
export const AdminSettingsPage = named(() => import("./admin/settings/AdminSettingsPage"), "AdminSettingsPage");
export const AdminAuditLogPage = named(() => import("./admin/auditlog/AdminAuditLogPage"), "AdminAuditLogPage");
