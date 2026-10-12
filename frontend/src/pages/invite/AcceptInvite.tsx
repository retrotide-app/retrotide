import { useNavigate } from "react-router";
import { Button } from "../../components/Button/Button";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { AuthLayout } from "../../components/layout/AuthLayout/AuthLayout";
import { readUpgradeRequired } from "../../domain/entitlements";
import { joinedOrganisationPath, organisationHomePath } from "../../domain/organisations";
import { useLogout } from "../../hooks/mutations/auth";
import { useAcceptInvite } from "../../hooks/mutations/invites";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import type { InviteDetailsResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";
import { errorCode, errorMessage } from "../../utils/errorMessage";
import authStyles from "../auth/AuthForm.module.css";
import { InviteDetails } from "./InviteDetails";
import { ACCEPT_TITLE, EMAIL_MISMATCH_CODE, INVITE_INVALID, INVITE_INVALID_CODE } from "./inviteMessages";
import { InviteUpgrade } from "./InviteUpgrade";

interface AcceptInviteProps {
    invite: InviteDetailsResponse;
    token: string;
    user: SessionUser;
    forget: () => void;
}

const EMAIL_UNVERIFIED_CODE = "email_unverified";

const ALREADY_MEMBER_CODE = "already_member";

const EMAIL_MISMATCH =
    "This invite is for a different email address. Sign out, then sign in with the address it was sent to.";

const EMAIL_UNVERIFIED = "Verify your email address before you accept this invite.";

const ACCEPT_FAILED = "Accepting the invite failed. Try again.";

const SIGN_OUT_FAILED = "Signing out failed. Try again.";

function sameAddress(first: string, second: string): boolean {
    return first.trim().toLowerCase() === second.trim().toLowerCase();
}

function acceptFailure(error: Error, invite: InviteDetailsResponse): string {
    const code = errorCode(error);

    if (code === INVITE_INVALID_CODE) {
        return INVITE_INVALID;
    }

    if (code === EMAIL_MISMATCH_CODE) {
        return EMAIL_MISMATCH;
    }

    if (code === EMAIL_UNVERIFIED_CODE) {
        return EMAIL_UNVERIFIED;
    }

    if (code === ALREADY_MEMBER_CODE) {
        return `You are already a member of ${invite.org_name}.`;
    }

    return errorMessage(error, ACCEPT_FAILED);
}

export function AcceptInvite({ invite, token, user, forget }: AcceptInviteProps) {
    const accept = useAcceptInvite();
    const logout = useLogout();
    const { selectOrganisation } = useCurrentOrg();
    const navigate = useNavigate();

    const code = errorCode(accept.error);
    const upgrade = readUpgradeRequired(accept.error);
    const boundAddress = invite.email ?? "";
    const boundElsewhere = boundAddress !== "" && !sameAddress(boundAddress, user.email);
    const settled = code === ALREADY_MEMBER_CODE || code === INVITE_INVALID_CODE;

    function handleAccept() {
        accept.mutate(
            { token },
            {
                onSuccess: joined => {
                    forget();
                    selectOrganisation(joined.org_id);
                    navigate(joinedOrganisationPath(joined), { replace: true });
                },
                onError: failure => {
                    const failed = errorCode(failure);

                    if (failed === INVITE_INVALID_CODE || failed === ALREADY_MEMBER_CODE) {
                        forget();
                    }
                },
            },
        );
    }

    function handleSignOut() {
        accept.reset();
        logout.mutate();
    }

    function handleNotNow() {
        forget();
        navigate("/", { replace: true });
    }

    function nextStep() {
        if (boundElsewhere || code === EMAIL_MISMATCH_CODE) {
            return (
                <Button
                    type="button"
                    variant="primary"
                    className={authStyles.wide}
                    onClick={handleSignOut}
                    disabled={logout.isPending}
                >
                    Sign out
                </Button>
            );
        }

        if (code === ALREADY_MEMBER_CODE) {
            return (
                <ButtonLink to={organisationHomePath(invite.org_slug)} variant="primary" className={authStyles.wide}>
                    Go to <bdi>{invite.org_name}</bdi>
                </ButtonLink>
            );
        }

        if (code === INVITE_INVALID_CODE) {
            return (
                <ButtonLink to="/" className={authStyles.wide}>
                    Go to the home page
                </ButtonLink>
            );
        }

        return (
            <Button
                type="button"
                variant="primary"
                className={authStyles.wide}
                onClick={handleAccept}
                disabled={accept.isPending}
            >
                Accept invite
            </Button>
        );
    }

    return (
        <AuthLayout title={ACCEPT_TITLE}>
            <InviteDetails invite={invite} />
            {upgrade !== null && <InviteUpgrade upgrade={upgrade} orgName={invite.org_name} />}
            {upgrade === null && accept.error !== null && (
                <ErrorBanner message={acceptFailure(accept.error, invite)} takeFocus />
            )}
            {logout.error !== null && <ErrorBanner message={errorMessage(logout.error, SIGN_OUT_FAILED)} takeFocus />}
            <p className={authStyles.lead}>
                You are signed in as <bdi>{user.email}</bdi>.
                {boundElsewhere && (
                    <>
                        {" "}
                        This invite is for <bdi>{boundAddress}</bdi>. Sign out, then sign in with that address to accept
                        it.
                    </>
                )}
            </p>
            <div className={authStyles.actions}>
                {nextStep()}
                {!settled && (
                    <Button
                        type="button"
                        variant="ghost"
                        className={authStyles.wide}
                        onClick={handleNotNow}
                        disabled={accept.isPending}
                    >
                        Not now
                    </Button>
                )}
            </div>
        </AuthLayout>
    );
}
