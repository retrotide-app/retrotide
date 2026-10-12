import type { ReactNode } from "react";
import { Navigate, Outlet } from "react-router";
import { INVITE_PATH, ONBOARDING_PATH } from "../../domain/organisations";
import { useAuth } from "../../hooks/useAuth";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import { useHasPendingInvite } from "../../hooks/useInviteToken";
import { LoadFailure } from "../LoadFailure/LoadFailure";
import { PageState } from "../PageState/PageState";

interface RequireOrganisationProps {
    children?: ReactNode;
}

export function RequireOrganisation({ children }: RequireOrganisationProps) {
    const { user, permissions } = useAuth();
    const { organisations, loading, error, retry } = useCurrentOrg();
    const hasPendingInvite = useHasPendingInvite();

    if (user === null) {
        return children ?? <Outlet />;
    }

    if (loading) {
        return <div className="loading">Loading...</div>;
    }

    if (error !== null) {
        return (
            <PageState title="We could not load your organisations">
                <LoadFailure message={error.message} onRetry={retry} />
            </PageState>
        );
    }

    if (organisations.length === 0 && hasPendingInvite) {
        return <Navigate to={INVITE_PATH} replace />;
    }

    if (organisations.length === 0 && permissions.includes("view_admin_panel")) {
        return <Navigate to="/admin" replace />;
    }

    if (organisations.length === 0) {
        return <Navigate to={ONBOARDING_PATH} replace />;
    }

    return children ?? <Outlet />;
}
