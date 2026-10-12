import { type SubmitEvent, useState } from "react";
import { Navigate } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { DataTable } from "../../../components/DataTable/DataTable";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Pagination } from "../../../components/Pagination/Pagination";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { Toast } from "../../../components/Toast/Toast";
import { emailDeliveryRefusal } from "../../../domain/refusals";
import { useCancelInvitation, useInviteUser, useResendInvitation } from "../../../hooks/mutations/admin";
import { useAdminInvitations } from "../../../hooks/queries/admin";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import type { AdminInvitationResponse } from "../../../types/api";
import { errorCode, errorStatus } from "../../../utils/errorMessage";
import { formatDate, formatExactDateTime } from "../../../utils/time";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminInvitationsPage.module.css";

const PAGE_SIZE = 20;

const EMAIL_MISSING = "Enter an email address.";

const ADDRESS_REFUSAL_CODES: ReadonlySet<string> = new Set(["account_exists", "invitation_pending"]);

function addressRefusal(error: Error | null): string | null {
    if (error === null) {
        return null;
    }

    if (errorStatus(error) === 400 || ADDRESS_REFUSAL_CODES.has(errorCode(error) ?? "")) {
        return error.message;
    }

    return null;
}

function InviteForm() {
    const invite = useInviteUser();
    const [email, setEmail] = useState("");
    const [problem, setProblem] = useState<string | null>(null);

    const summary = problem ?? emailDeliveryRefusal(invite.error) ?? invite.error?.message ?? null;
    const fieldError = problem ?? addressRefusal(invite.error);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        const trimmed = email.trim();

        if (trimmed === "") {
            invite.reset();
            setProblem(EMAIL_MISSING);
            return;
        }

        setProblem(null);
        invite.mutate(
            { email: trimmed },
            {
                onSuccess: () => {
                    setEmail("");
                },
            },
        );
    }

    return (
        <>
            <form className={styles.invite} onSubmit={handleSubmit} noValidate>
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <div className={styles.inviteRow}>
                    <div className={styles.inviteField}>
                        <TextField
                            label="Email address"
                            hint="We email a link that lets them create an account. It works for 7 days."
                            type="email"
                            value={email}
                            onChange={e => setEmail(e.target.value)}
                            error={fieldError}
                            autoComplete="off"
                        />
                    </div>
                    <div className={styles.inviteAction}>
                        <Button type="submit" variant="primary" disabled={invite.isPending}>
                            Send invitation
                        </Button>
                    </div>
                </div>
            </form>
            {invite.isSuccess && (
                <Toast variant="success" onDismiss={invite.reset}>
                    Invitation sent.
                </Toast>
            )}
        </>
    );
}

function InvitationList() {
    const [offset, setOffset] = useState(0);
    const invitations = useAdminInvitations({ limit: PAGE_SIZE, offset });
    const resend = useResendInvitation();
    const cancel = useCancelInvitation();
    const [cancelling, setCancelling] = useState<AdminInvitationResponse | null>(null);

    const listing = invitations.data;

    function openCancel(invitation: AdminInvitationResponse) {
        cancel.reset();
        setCancelling(invitation);
    }

    function closeCancel() {
        cancel.reset();
        setCancelling(null);
    }

    function handleCancel() {
        if (cancelling === null) {
            return;
        }

        cancel.mutate(
            { invitationId: cancelling.id },
            {
                onSuccess: () => {
                    setCancelling(null);
                },
            },
        );
    }

    const dialogBody = (
        <div className={styles.dialog}>
            <p>
                The invitation to <bdi>{cancelling?.email}</bdi> stops working.
            </p>
            {cancel.error !== null && <ErrorBanner message={cancel.error.message} takeFocus />}
        </div>
    );

    return (
        <>
            {resend.error !== null && (
                <ErrorBanner message={emailDeliveryRefusal(resend.error) ?? resend.error.message} takeFocus />
            )}

            {invitations.error !== null && (
                <LoadFailure message={invitations.error.message} onRetry={invitations.refetch} />
            )}

            {listing === undefined && invitations.error === null && <p className="loading">Loading invitations...</p>}

            {listing !== undefined && listing.total === 0 && <p className={styles.empty}>No pending invitations.</p>}

            {listing !== undefined && listing.invitations.length > 0 && (
                <DataTable caption="Pending invitations">
                    <thead>
                        <tr>
                            <th scope="col">Email</th>
                            <th scope="col">Status</th>
                            <th scope="col">Sent</th>
                            <th scope="col">Expires</th>
                            <th scope="col">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {listing.invitations.map(invitation => (
                            <tr key={invitation.id}>
                                <td>
                                    <bdi>{invitation.email}</bdi>
                                </td>
                                <td>
                                    {invitation.expired ? (
                                        <StatusTag tone="warning">Expired</StatusTag>
                                    ) : (
                                        <StatusTag tone="neutral">Pending</StatusTag>
                                    )}
                                </td>
                                <td>
                                    <time dateTime={invitation.sent_at}>{formatDate(invitation.sent_at)}</time>
                                </td>
                                <td className={styles.when}>
                                    <time dateTime={invitation.expires_at}>
                                        {formatExactDateTime(invitation.expires_at)}
                                    </time>
                                </td>
                                <td>
                                    <div className={styles.actions}>
                                        <Button
                                            type="button"
                                            size="small"
                                            aria-label={`Resend invitation to ${invitation.email}`}
                                            disabled={resend.isPending}
                                            onClick={() => resend.mutate({ invitationId: invitation.id })}
                                        >
                                            Resend
                                        </Button>
                                        <Button
                                            type="button"
                                            size="small"
                                            variant="ghost"
                                            aria-label={`Cancel invitation to ${invitation.email}`}
                                            onClick={() => openCancel(invitation)}
                                        >
                                            Cancel
                                        </Button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}

            {listing !== undefined && (
                <Pagination
                    label="Invitation pages"
                    offset={listing.offset}
                    limit={listing.limit}
                    total={listing.total}
                    onChange={setOffset}
                />
            )}

            <ConfirmDialog
                open={cancelling !== null}
                title="Cancel invitation"
                body={dialogBody}
                confirmLabel="Cancel invitation"
                cancelLabel="Keep invitation"
                destructive
                busy={cancel.isPending}
                onConfirm={handleCancel}
                onCancel={closeCancel}
            />

            {resend.isSuccess && (
                <Toast variant="success" onDismiss={resend.reset}>
                    Invitation sent again.
                </Toast>
            )}

            {cancel.isSuccess && (
                <Toast variant="success" onDismiss={cancel.reset}>
                    Invitation cancelled.
                </Toast>
            )}
        </>
    );
}

export function AdminInvitationsPage() {
    const { site_mode } = useSiteInfo();

    if (site_mode !== "private") {
        return <Navigate to="/admin" replace />;
    }

    return (
        <AdminPage
            title="Invitations"
            description="This site is private, so people can only join when you invite them by email."
        >
            <InviteForm />
            <InvitationList />
        </AdminPage>
    );
}
