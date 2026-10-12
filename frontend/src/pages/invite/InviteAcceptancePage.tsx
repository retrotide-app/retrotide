import { useEffect } from "react";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { AuthLayout } from "../../components/layout/AuthLayout/AuthLayout";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { SessionCheckFailure } from "../../components/SessionCheckFailure/SessionCheckFailure";
import { useInviteLookup } from "../../hooks/queries/invites";
import { useAuth } from "../../hooks/useAuth";
import { useInviteToken } from "../../hooks/useInviteToken";
import { usePageTitle } from "../../hooks/usePageTitle";
import { errorCode, errorMessage } from "../../utils/errorMessage";
import authStyles from "../auth/AuthForm.module.css";
import { AcceptInvite } from "./AcceptInvite";
import { ACCEPT_TITLE, INVITE_INVALID, INVITE_INVALID_CODE } from "./inviteMessages";
import { InviteSignUp } from "./InviteSignUp";
import styles from "./InviteAcceptancePage.module.css";

interface InviteRefusedProps {
    message: string;
}

const MISSING_TOKEN = "This link is missing its code. Open the invite link again.";

const LOOKUP_FAILED = "Your invite could not be read.";

function InviteRefused({ message }: InviteRefusedProps) {
    return (
        <AuthLayout title={ACCEPT_TITLE}>
            <div className={styles.stack}>
                <ErrorBanner message={message} takeFocus />
                <ButtonLink to="/" className={authStyles.wide}>
                    Go to the home page
                </ButtonLink>
            </div>
        </AuthLayout>
    );
}

export function InviteAcceptancePage() {
    usePageTitle(ACCEPT_TITLE);

    const { token, forget } = useInviteToken();
    const lookup = useInviteLookup(token);
    const { user, loading, error, retry } = useAuth();

    const refused = errorCode(lookup.error) === INVITE_INVALID_CODE;

    useEffect(() => {
        if (!refused) {
            return;
        }

        forget();
    }, [refused, forget]);

    if (token === "") {
        return <InviteRefused message={MISSING_TOKEN} />;
    }

    const invite = lookup.data;

    if (invite === undefined && refused) {
        return <InviteRefused message={INVITE_INVALID} />;
    }

    if (invite === undefined && lookup.error !== null) {
        return (
            <AuthLayout title={ACCEPT_TITLE}>
                <LoadFailure message={errorMessage(lookup.error, LOOKUP_FAILED)} onRetry={lookup.refetch} />
            </AuthLayout>
        );
    }

    if (invite === undefined) {
        return (
            <AuthLayout title={ACCEPT_TITLE}>
                <p role="status" className={authStyles.lead}>
                    Checking your invite...
                </p>
            </AuthLayout>
        );
    }

    if (user !== null) {
        return <AcceptInvite invite={invite} token={token} user={user} forget={forget} />;
    }

    if (loading) {
        return (
            <AuthLayout title={ACCEPT_TITLE}>
                <p role="status" className={authStyles.lead}>
                    Checking your sign-in...
                </p>
            </AuthLayout>
        );
    }

    if (error !== null) {
        return (
            <AuthLayout title={ACCEPT_TITLE}>
                <SessionCheckFailure message={error.message} onRetry={retry} headingLevel={2} />
            </AuthLayout>
        );
    }

    return <InviteSignUp invite={invite} token={token} forget={forget} />;
}
