import type { ReactNode } from "react";
import { Navigate, Outlet, useLocation } from "react-router";
import type { StaffPermission } from "../../domain/permissions";
import { returnState } from "../../domain/returnTo";
import { useAuth } from "../../hooks/useAuth";
import { ButtonLink } from "../Button/ButtonLink";
import { type PageHeadingLevel, SessionCheckFailure } from "../SessionCheckFailure/SessionCheckFailure";
import styles from "./ProtectedRoute.module.css";

interface ProtectedRouteProps {
    permission?: StaffPermission;
    headingLevel?: PageHeadingLevel;
    children?: ReactNode;
}

const DENIED_TITLE = "You cannot open this page";

export function ProtectedRoute({ permission, headingLevel = 1, children }: ProtectedRouteProps) {
    const { user, permissions, loading, error, retry } = useAuth();
    const location = useLocation();

    if (loading) {
        return <div className="loading">Loading...</div>;
    }

    if (user === null && error !== null) {
        return <SessionCheckFailure message={error.message} onRetry={retry} headingLevel={headingLevel} />;
    }

    if (user === null) {
        return <Navigate to="/login" replace state={returnState(location.pathname, location.search)} />;
    }

    if (permission !== undefined && !permissions.includes(permission)) {
        return (
            <section className={styles.state} aria-labelledby="access-denied-title">
                {headingLevel === 1 ? (
                    <h1 id="access-denied-title" className={styles.title}>
                        {DENIED_TITLE}
                    </h1>
                ) : (
                    <h2 id="access-denied-title" className={styles.title}>
                        {DENIED_TITLE}
                    </h2>
                )}
                <p className={styles.blurb}>Your account does not have access to it.</p>
                <ButtonLink to="/">Go to the home page</ButtonLink>
            </section>
        );
    }

    return children ?? <Outlet />;
}
