import { useState } from "react";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { DataTable } from "../../../components/DataTable/DataTable";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Menu, type MenuItem } from "../../../components/Menu/Menu";
import { Panel } from "../../../components/Panel/Panel";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { Toast } from "../../../components/Toast/Toast";
import { canManageInvite, ORG_ROLE_LABELS } from "../../../domain/organisationRoles";
import { useResendInvite, useRevokeInvite } from "../../../hooks/mutations/invites";
import { useOrganisationInvites } from "../../../hooks/queries/invites";
import type { OrganisationInviteResponse, OrganisationResponse, OrgRole, TeamResponse } from "../../../types/api";
import { formatExactDateTime } from "../../../utils/time";
import { ActionFailure } from "../shared/ActionFailure";
import styles from "../shared/OrgSection.module.css";

interface InviteListProps {
    organisation: OrganisationResponse;
    role: OrgRole | null;
    teams: readonly TeamResponse[];
}

function inviteName(invite: OrganisationInviteResponse): string {
    if (invite.email !== undefined) {
        return invite.email;
    }

    return `the invite link that expires ${formatExactDateTime(invite.expires_at)}`;
}

function usesOf(invite: OrganisationInviteResponse): string {
    if (invite.max_uses === undefined) {
        return `${invite.use_count} (no limit)`;
    }

    return `${invite.use_count} of ${invite.max_uses}`;
}

export function InviteList({ organisation, role, teams }: InviteListProps) {
    const invites = useOrganisationInvites(organisation.id);
    const resend = useResendInvite();
    const revoke = useRevokeInvite();
    const [revoking, setRevoking] = useState<OrganisationInviteResponse | null>(null);
    const [resentTo, setResentTo] = useState("");

    const listed = invites.data?.invites ?? [];
    const acting = listed.some(invite => canManageInvite(role, invite));
    const teamNames = new Map(teams.map(team => [team.id, team.name]));

    function teamOf(invite: OrganisationInviteResponse): string {
        if (invite.team_id === undefined) {
            return "No team";
        }

        return teamNames.get(invite.team_id) ?? "Unknown team";
    }

    function handleResend(invite: OrganisationInviteResponse) {
        setResentTo(invite.email ?? "");
        resend.mutate({ orgId: organisation.id, inviteId: invite.id });
    }

    function openRevoke(invite: OrganisationInviteResponse) {
        revoke.reset();
        setRevoking(invite);
    }

    function closeRevoke() {
        revoke.reset();
        setRevoking(null);
    }

    function menuItems(invite: OrganisationInviteResponse): MenuItem[] {
        const resendItem: MenuItem[] =
            invite.email === undefined || resend.isPending
                ? []
                : [{ label: "Resend", onSelect: () => handleResend(invite) }];

        return [...resendItem, { label: "Revoke", tone: "danger", onSelect: () => openRevoke(invite) }];
    }

    function handleRevoke() {
        if (revoking === null) {
            return;
        }

        revoke.mutate(
            { orgId: organisation.id, inviteId: revoking.id },
            {
                onSuccess: () => {
                    setRevoking(null);
                },
            },
        );
    }

    const revokeBody = (
        <div className={styles.dialog}>
            {revoking?.email === undefined ? (
                <p>The invite link stops working, so nobody else can join with it.</p>
            ) : (
                <p>
                    The invite to <bdi>{revoking.email}</bdi> stops working.
                </p>
            )}
            {revoke.error !== null && <ActionFailure error={revoke.error} takeFocus />}
        </div>
    );

    return (
        <Panel title="Pending invites" headingLevel={2}>
            {resend.error !== null && <ActionFailure error={resend.error} takeFocus />}

            {invites.error !== null && <LoadFailure message={invites.error.message} onRetry={invites.refetch} />}

            {invites.data === undefined && invites.error === null && <p className="loading">Loading invites...</p>}

            {invites.data !== undefined && listed.length === 0 && <p>No pending invites.</p>}

            {listed.length > 0 && (
                <DataTable caption="Pending invites">
                    <thead>
                        <tr>
                            <th scope="col">Invite</th>
                            <th scope="col">Role</th>
                            <th scope="col">Team</th>
                            <th scope="col">Uses</th>
                            <th scope="col">Expires</th>
                            {acting && <th scope="col">Actions</th>}
                        </tr>
                    </thead>
                    <tbody>
                        {listed.map(invite => (
                            <tr key={invite.id}>
                                <td>
                                    {invite.email === undefined ? <span>Invite link</span> : <bdi>{invite.email}</bdi>}
                                </td>
                                <td>{ORG_ROLE_LABELS[invite.role]}</td>
                                <td>
                                    <bdi>{teamOf(invite)}</bdi>
                                </td>
                                <td>{usesOf(invite)}</td>
                                <td className={styles.when}>
                                    <div className={styles.inline}>
                                        {invite.expired && <StatusTag tone="warning">Expired</StatusTag>}
                                        <time dateTime={invite.expires_at}>
                                            {formatExactDateTime(invite.expires_at)}
                                        </time>
                                    </div>
                                </td>
                                {acting && (
                                    <td>
                                        {canManageInvite(role, invite) && (
                                            <Menu
                                                label={`Actions for ${inviteName(invite)}`}
                                                items={menuItems(invite)}
                                                iconOnly
                                                size="small"
                                            />
                                        )}
                                    </td>
                                )}
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}

            <ConfirmDialog
                open={revoking !== null}
                title="Revoke this invite?"
                body={revokeBody}
                confirmLabel="Revoke"
                destructive
                busy={revoke.isPending}
                onConfirm={handleRevoke}
                onCancel={closeRevoke}
            />

            {resend.isSuccess && (
                <Toast variant="success" onDismiss={resend.reset}>
                    Invite sent again to <bdi>{resentTo}</bdi>.
                </Toast>
            )}

            {revoke.isSuccess && (
                <Toast variant="success" onDismiss={revoke.reset}>
                    Invite revoked.
                </Toast>
            )}
        </Panel>
    );
}
