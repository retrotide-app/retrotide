import { type ReactNode, useCallback, useId, useState } from "react";
import { Button } from "../../components/Button/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { Menu, type MenuItem } from "../../components/Menu/Menu";
import { StatusTag } from "../../components/StatusTag/StatusTag";
import { Toast } from "../../components/Toast/Toast";
import { MEMBER_NOUN } from "../../domain/board/organisationBoards";
import { accountInitials } from "../../domain/displayName";
import {
    orgRoleGrants,
    TEAM_ROLE_LABELS,
    teamMemberActions,
    type TeamMemberActions,
    type TeamViewer,
} from "../../domain/organisationRoles";
import { useChangeTeamMemberRole, useRemoveTeamMember } from "../../hooks/mutations/teams";
import { useTeamMembers } from "../../hooks/queries/teams";
import type { OrganisationResponse, TeamMemberResponse, TeamResponse, TeamRole } from "../../types/api";
import { countOf } from "../../utils/chart";
import { formatDate } from "../../utils/time";
import { ActionFailure } from "../org/shared/ActionFailure";
import { AddPeopleDialog } from "./AddPeopleDialog";
import styles from "./TeamMembersSection.module.css";
import pageStyles from "./TeamPage.module.css";

interface TeamMembersSectionProps {
    organisation: OrganisationResponse;
    team: TeamResponse;
    viewer: TeamViewer;
    managesMembers: boolean;
}

interface MemberRow {
    member: TeamMemberResponse;
    actions: TeamMemberActions;
}

const NO_ACTIONS: TeamMemberActions = { roles: [], remove: false };

const ROLE_WITH_ARTICLE: Readonly<Record<TeamRole, string>> = {
    lead: "a lead",
    member: "a member",
};

export function TeamMembersSection({ organisation, team, viewer, managesMembers }: TeamMembersSectionProps) {
    const headingId = useId();
    const members = useTeamMembers(team.id);
    const changeRole = useChangeTeamMemberRole();
    const removal = useRemoveTeamMember();
    const [adding, setAdding] = useState(false);
    const [notice, setNotice] = useState<ReactNode>(null);
    const [removing, setRemoving] = useState<TeamMemberResponse | null>(null);
    const [removedName, setRemovedName] = useState("");
    const clearNotice = useCallback(() => setNotice(null), []);

    const loaded = members.data !== undefined;
    const listed = members.data?.members ?? [];

    const rows: MemberRow[] = listed.map(member => {
        if (!managesMembers) {
            return { member, actions: NO_ACTIONS };
        }

        return {
            member,
            actions: teamMemberActions(viewer, {
                userId: member.user_id,
                orgRole: member.org_role,
                teamRole: member.role,
                siteRole: member.site_role ?? null,
            }),
        };
    });

    const changed = changeRole.variables;
    const changedName = listed.find(member => member.user_id === changed?.userId)?.display_name ?? "";

    function menuItems({ member, actions }: MemberRow): MenuItem[] {
        const roleItems: MenuItem[] = actions.roles.map(role => ({
            label: `Make ${TEAM_ROLE_LABELS[role].toLowerCase()}`,
            onSelect: () => {
                changeRole.mutate({ teamId: team.id, userId: member.user_id, role });
            },
        }));

        if (!actions.remove) {
            return roleItems;
        }

        return [
            ...roleItems,
            {
                label: "Remove",
                tone: "danger",
                onSelect: () => {
                    removal.reset();
                    setRemoving(member);
                },
            },
        ];
    }

    function handleRemove() {
        if (removing === null) {
            return;
        }

        setRemovedName(removing.display_name);
        removal.mutate(
            { teamId: team.id, userId: removing.user_id },
            {
                onSuccess: () => {
                    setRemoving(null);
                },
            },
        );
    }

    function handleAdded(added: ReactNode) {
        setAdding(false);
        setNotice(added);
    }

    const removeBody = (
        <div className={pageStyles.dialogBody}>
            <p>
                <bdi>{removing?.display_name}</bdi> leaves <bdi>{team.name}</bdi> but stays in{" "}
                <bdi>{organisation.name}</bdi>.
            </p>
            {removal.error !== null && <ActionFailure error={removal.error} takeFocus />}
        </div>
    );

    return (
        <section aria-labelledby={headingId} className={pageStyles.section}>
            <div className={pageStyles.sectionHeader}>
                <div className={pageStyles.sectionHeading}>
                    <h2 id={headingId} className={pageStyles.sectionTitle}>
                        Members
                    </h2>
                    {loaded && <span className={pageStyles.count}>{countOf(listed.length, MEMBER_NOUN)}</span>}
                </div>
                {managesMembers && orgRoleGrants(viewer.orgRole, "org.manage_members") && (
                    <div className={pageStyles.sectionActions}>
                        <Button type="button" onClick={() => setAdding(true)} disabled={!loaded}>
                            Add people
                        </Button>
                    </div>
                )}
                <p className={pageStyles.hint}>Organisation owners and admins lead every team.</p>
            </div>

            {changeRole.error !== null && <ActionFailure error={changeRole.error} takeFocus />}

            {members.error !== null && <LoadFailure message={members.error.message} onRetry={members.refetch} />}

            {!loaded && members.error === null && <p className="loading">Loading members...</p>}

            {loaded && listed.length === 0 && <p>Nobody is on this team yet.</p>}

            {rows.length > 0 && (
                <ul className={styles.list} aria-label={`Members of ${team.name}`}>
                    {rows.map(row => {
                        const { member } = row;
                        const you = member.user_id === viewer.userId;

                        return (
                            <li key={member.user_id} className={styles.row}>
                                <span className={styles.avatar} aria-hidden="true">
                                    {accountInitials(member.display_name, member.email)}
                                </span>
                                <span className={styles.who}>
                                    <span className={styles.name}>
                                        <bdi>{member.display_name}</bdi>
                                        {you && " (you)"}
                                    </span>
                                    <span className={styles.email}>
                                        <bdi>{member.email}</bdi>
                                    </span>
                                </span>
                                <span className={styles.meta}>
                                    <StatusTag tone={member.role === "lead" ? "success" : "neutral"}>
                                        {TEAM_ROLE_LABELS[member.role]}
                                    </StatusTag>
                                    <span className={styles.joined}>
                                        Joined <time dateTime={member.joined_at}>{formatDate(member.joined_at)}</time>
                                    </span>
                                </span>
                                <span className={styles.actions}>
                                    <Menu
                                        label={`Actions for ${member.display_name}`}
                                        items={menuItems(row)}
                                        iconOnly
                                        size="small"
                                    />
                                </span>
                            </li>
                        );
                    })}
                </ul>
            )}

            {managesMembers && loaded && (
                <AddPeopleDialog
                    open={adding}
                    organisation={organisation}
                    team={team}
                    viewer={viewer}
                    members={listed}
                    onDone={handleAdded}
                    onClose={() => setAdding(false)}
                />
            )}

            <ConfirmDialog
                open={removing !== null}
                title={`Remove ${removing?.display_name ?? ""} from ${team.name}?`}
                body={removeBody}
                confirmLabel="Remove"
                destructive
                busy={removal.isPending}
                onConfirm={handleRemove}
                onCancel={() => {
                    removal.reset();
                    setRemoving(null);
                }}
            />

            {notice !== null && (
                <Toast variant="success" onDismiss={clearNotice}>
                    {notice}
                </Toast>
            )}

            {changeRole.isSuccess && changed !== undefined && (
                <Toast variant="success" onDismiss={changeRole.reset}>
                    <bdi>{changedName}</bdi> is now {ROLE_WITH_ARTICLE[changed.role]}.
                </Toast>
            )}

            {removal.isSuccess && (
                <Toast variant="success" onDismiss={removal.reset}>
                    <bdi>{removedName}</bdi> removed from <bdi>{team.name}</bdi>.
                </Toast>
            )}
        </section>
    );
}
