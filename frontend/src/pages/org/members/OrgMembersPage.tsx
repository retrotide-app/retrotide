import { useState } from "react";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { DataTable } from "../../../components/DataTable/DataTable";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Menu, type MenuItem } from "../../../components/Menu/Menu";
import { Toast } from "../../../components/Toast/Toast";
import {
    canLeaveOrganisation,
    ORG_ROLE_LABELS,
    orgMemberActions,
    orgRoleGrants,
    type OrgMemberActions,
    type OrgViewer,
} from "../../../domain/organisationRoles";
import { isDeleted } from "../../../domain/organisations";
import { refusedWith } from "../../../domain/refusals";
import {
    useChangeMemberRole,
    useLeaveOrganisation,
    useRemoveMember,
    useTransferOwnership,
} from "../../../hooks/mutations/organisations";
import { useOrganisationMembers } from "../../../hooks/queries/organisations";
import { useAuth } from "../../../hooks/useAuth";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import type { AssignableOrgRole, OrganisationMemberResponse, OrganisationResponse } from "../../../types/api";
import { formatDate } from "../../../utils/time";
import { ActionFailure } from "../shared/ActionFailure";
import { DeletedOrganisation } from "../shared/DeletedOrganisation";
import { OrgPage } from "../shared/OrgPage";
import styles from "../shared/OrgSection.module.css";

const PAGE_TITLE = "Members";

const ROLE_WITH_ARTICLE: Readonly<Record<AssignableOrgRole, string>> = {
    admin: "an admin",
    member: "a member",
};

interface MemberListProps {
    organisation: OrganisationResponse;
    viewer: OrgViewer;
}

interface MemberRow {
    member: OrganisationMemberResponse;
    actions: OrgMemberActions;
}

function hasActions(actions: OrgMemberActions): boolean {
    return actions.roles.length > 0 || actions.remove || actions.transferOwnership;
}

function nameOf(members: readonly OrganisationMemberResponse[], userId: string | undefined): string {
    return members.find(member => member.user_id === userId)?.display_name ?? "";
}

function transferRefusal(error: Error, newOwner: string): string {
    if (refusedWith(error, ["new_owner_limit_reached"])) {
        return `${newOwner} already owns as many organisations as they can, so you stay the owner.`;
    }

    return error.message;
}

function MemberList({ organisation, viewer }: MemberListProps) {
    const members = useOrganisationMembers(organisation.id);
    const changeRole = useChangeMemberRole();
    const removal = useRemoveMember();
    const transfer = useTransferOwnership();
    const leave = useLeaveOrganisation();
    const [removing, setRemoving] = useState<OrganisationMemberResponse | null>(null);
    const [removedName, setRemovedName] = useState("");
    const [promoting, setPromoting] = useState<OrganisationMemberResponse | null>(null);
    const [leaving, setLeaving] = useState(false);

    const listed = members.data?.members ?? [];
    const rows: MemberRow[] = listed.map(member => ({ member, actions: orgMemberActions(viewer, member) }));
    const leaves = canLeaveOrganisation(viewer.role);
    const acting = leaves || rows.some(row => hasActions(row.actions));

    const changed = changeRole.variables;
    const changedName = nameOf(listed, changed?.userId);
    const promotedName = nameOf(listed, transfer.variables?.user_id);

    function openRemove(member: OrganisationMemberResponse) {
        removal.reset();
        setRemoving(member);
    }

    function closeRemove() {
        removal.reset();
        setRemoving(null);
    }

    function handleRemove() {
        if (removing === null) {
            return;
        }

        setRemovedName(removing.display_name);
        removal.mutate(
            { orgId: organisation.id, userId: removing.user_id },
            {
                onSuccess: () => {
                    setRemoving(null);
                },
            },
        );
    }

    function openTransfer(member: OrganisationMemberResponse) {
        transfer.reset();
        setPromoting(member);
    }

    function closeTransfer() {
        transfer.reset();
        setPromoting(null);
    }

    function handleTransfer() {
        if (promoting === null) {
            return;
        }

        transfer.mutate(
            { orgId: organisation.id, user_id: promoting.user_id },
            {
                onSuccess: () => {
                    setPromoting(null);
                },
            },
        );
    }

    function openLeave() {
        leave.reset();
        setLeaving(true);
    }

    function menuItems(member: OrganisationMemberResponse, actions: OrgMemberActions): MenuItem[] {
        const roles: MenuItem[] = changeRole.isPending
            ? []
            : actions.roles.map(role => ({
                  label: `Make ${ORG_ROLE_LABELS[role].toLowerCase()}`,
                  onSelect: () => {
                      changeRole.mutate({ orgId: organisation.id, userId: member.user_id, role });
                  },
              }));
        const transferItem: MenuItem[] = actions.transferOwnership
            ? [{ label: "Make owner", onSelect: () => openTransfer(member) }]
            : [];
        const removeItem: MenuItem[] = actions.remove
            ? [{ label: "Remove", tone: "danger", onSelect: () => openRemove(member) }]
            : [];
        const leaveItem: MenuItem[] =
            leaves && member.user_id === viewer.userId
                ? [{ label: "Leave organisation", tone: "danger", onSelect: openLeave }]
                : [];

        return [...roles, ...transferItem, ...removeItem, ...leaveItem];
    }

    function closeLeave() {
        leave.reset();
        setLeaving(false);
    }

    function handleLeave() {
        leave.mutate(
            { orgId: organisation.id },
            {
                onSuccess: () => {
                    setLeaving(false);
                },
            },
        );
    }

    const removeBody = (
        <div className={styles.dialog}>
            <p>
                <bdi>{removing?.display_name}</bdi> loses access to <bdi>{organisation.name}</bdi> and its teams.
            </p>
            {removal.error !== null && <ActionFailure error={removal.error} takeFocus />}
        </div>
    );

    const transferBody = (
        <div className={styles.dialog}>
            <p>
                <bdi>{promoting?.display_name}</bdi> becomes the owner of <bdi>{organisation.name}</bdi>. You become an
                admin, so you can no longer delete the organisation or make someone else the owner.
            </p>
            {transfer.error !== null && (
                <ErrorBanner message={transferRefusal(transfer.error, promoting?.display_name ?? "")} takeFocus />
            )}
        </div>
    );

    const leaveBody = (
        <div className={styles.dialog}>
            <p>
                You lose access to <bdi>{organisation.name}</bdi> and its teams. To come back, you need a new invite.
            </p>
            {leave.error !== null && <ActionFailure error={leave.error} takeFocus />}
        </div>
    );

    return (
        <>
            {changeRole.error !== null && <ActionFailure error={changeRole.error} takeFocus />}

            {members.error !== null && <LoadFailure message={members.error.message} onRetry={members.refetch} />}

            {members.data === undefined && members.error === null && <p className="loading">Loading members...</p>}

            {rows.length > 0 && (
                <DataTable caption="Members">
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Email</th>
                            <th scope="col">Role</th>
                            <th scope="col">Joined</th>
                            {acting && <th scope="col">Actions</th>}
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map(({ member, actions }) => (
                            <tr key={member.user_id}>
                                <td>
                                    <bdi>{member.display_name}</bdi>
                                    {member.user_id === viewer.userId && " (you)"}
                                </td>
                                <td>
                                    <bdi>{member.email}</bdi>
                                </td>
                                <td>{ORG_ROLE_LABELS[member.role]}</td>
                                <td className={styles.when}>
                                    <time dateTime={member.joined_at}>{formatDate(member.joined_at)}</time>
                                </td>
                                {acting && (
                                    <td>
                                        <Menu
                                            label={`Actions for ${member.display_name}`}
                                            items={menuItems(member, actions)}
                                            iconOnly
                                            size="small"
                                        />
                                    </td>
                                )}
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}

            <ConfirmDialog
                open={removing !== null}
                title={`Remove ${removing?.display_name ?? ""}?`}
                body={removeBody}
                confirmLabel="Remove"
                destructive
                busy={removal.isPending}
                onConfirm={handleRemove}
                onCancel={closeRemove}
            />

            <ConfirmDialog
                open={promoting !== null}
                title={`Make ${promoting?.display_name ?? ""} the owner?`}
                body={transferBody}
                confirmLabel="Make owner"
                destructive
                busy={transfer.isPending}
                onConfirm={handleTransfer}
                onCancel={closeTransfer}
            />

            <ConfirmDialog
                open={leaving}
                title={`Leave ${organisation.name}?`}
                body={leaveBody}
                confirmLabel="Leave organisation"
                destructive
                busy={leave.isPending}
                onConfirm={handleLeave}
                onCancel={closeLeave}
            />

            {changeRole.isSuccess && changed !== undefined && (
                <Toast variant="success" onDismiss={changeRole.reset}>
                    <bdi>{changedName}</bdi> is now {ROLE_WITH_ARTICLE[changed.role]}.
                </Toast>
            )}

            {transfer.isSuccess && (
                <Toast variant="success" onDismiss={transfer.reset}>
                    <bdi>{promotedName}</bdi> is now the owner.
                </Toast>
            )}

            {removal.isSuccess && (
                <Toast variant="success" onDismiss={removal.reset}>
                    <bdi>{removedName}</bdi> removed.
                </Toast>
            )}

            {leave.isSuccess && (
                <Toast variant="success" onDismiss={leave.reset}>
                    You left <bdi>{organisation.name}</bdi>.
                </Toast>
            )}
        </>
    );
}

export function OrgMembersPage() {
    const { current, role } = useCurrentOrg();
    const { user, siteRole } = useAuth();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    if (!orgRoleGrants(role, "org.manage_members")) {
        return (
            <OrgPage title={PAGE_TITLE}>
                <p>Only the owner and admins can see who is in the organisation.</p>
            </OrgPage>
        );
    }

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="Everyone in your organisation and their role. Owners and admins can change roles and remove people."
        >
            <MemberList key={current.id} organisation={current} viewer={{ userId: user?.id ?? "", role, siteRole }} />
        </OrgPage>
    );
}
