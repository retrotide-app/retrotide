import type { ReactNode } from "react";
import { useParams } from "react-router";
import { DataTable } from "../../../components/DataTable/DataTable";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Panel } from "../../../components/Panel/Panel";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { TextLink } from "../../../components/TextLink/TextLink";
import { PLAN_LABELS } from "../../../domain/entitlements";
import { ORG_ROLE_LABELS } from "../../../domain/organisationRoles";
import { staffRoleLabel } from "../../../domain/staffRoles";
import { useAdminOrganisation } from "../../../hooks/queries/adminOrganisations";
import type { AdminOrganisationDetailResponse } from "../../../types/api";
import { formatDate, formatExactDateTime } from "../../../utils/time";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminOrganisationPage.module.css";
import { UserName } from "./UserName";

interface OrganisationProps {
    organisation: AdminOrganisationDetailResponse;
}

interface DetailProps {
    term: string;
    children: ReactNode;
}

function Detail({ term, children }: DetailProps) {
    return (
        <div className={styles.item}>
            <dt className={styles.term}>{term}</dt>
            <dd className={styles.value}>{children}</dd>
        </div>
    );
}

function OrganisationDetails({ organisation }: OrganisationProps) {
    return (
        <Panel title="Details" headingLevel={3}>
            <dl className={styles.details}>
                <Detail term="Slug">
                    <bdi>{organisation.slug}</bdi>
                </Detail>
                <Detail term="Plan">{PLAN_LABELS[organisation.plan]}</Detail>
                <Detail term="Owner">
                    {organisation.owner_id === undefined ? (
                        "No owner"
                    ) : (
                        <span className={styles.stack}>
                            <UserName userId={organisation.owner_id} name={organisation.owner_display_name ?? ""} />
                            <bdi className={styles.muted}>{organisation.owner_email}</bdi>
                        </span>
                    )}
                </Detail>
                <Detail term="Status">
                    {organisation.deleted_at === undefined ? (
                        <StatusTag tone="success">Active</StatusTag>
                    ) : (
                        <StatusTag tone="danger">Deleted</StatusTag>
                    )}
                </Detail>
                <Detail term="Created">{formatExactDateTime(organisation.created_at)}</Detail>
                {organisation.deleted_at !== undefined && (
                    <Detail term="Deleted on">{formatExactDateTime(organisation.deleted_at)}</Detail>
                )}
                <Detail term="Members">{organisation.member_count}</Detail>
                <Detail term="Teams">{organisation.team_count}</Detail>
                <Detail term="Boards">{organisation.board_count}</Detail>
            </dl>
        </Panel>
    );
}

function OrganisationTeams({ organisation }: OrganisationProps) {
    return (
        <Panel title="Teams" headingLevel={3}>
            {organisation.teams.length === 0 ? (
                <p className={styles.muted}>No teams yet.</p>
            ) : (
                <DataTable caption="Teams">
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Members</th>
                            <th scope="col">Boards</th>
                            <th scope="col">Created</th>
                            <th scope="col">Status</th>
                        </tr>
                    </thead>
                    <tbody>
                        {organisation.teams.map(team => (
                            <tr key={team.id}>
                                <td>
                                    <bdi>{team.name}</bdi>
                                </td>
                                <td className={styles.number}>{team.member_count}</td>
                                <td className={styles.number}>{team.board_count}</td>
                                <td>{formatDate(team.created_at)}</td>
                                <td>
                                    {team.archived_at === undefined ? (
                                        <StatusTag tone="success">Active</StatusTag>
                                    ) : (
                                        <StatusTag tone="neutral">Archived</StatusTag>
                                    )}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}
        </Panel>
    );
}

function OrganisationMembers({ organisation }: OrganisationProps) {
    return (
        <Panel title="Members" headingLevel={3}>
            {organisation.members.length === 0 ? (
                <p className={styles.muted}>No members.</p>
            ) : (
                <DataTable caption="Members">
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Email</th>
                            <th scope="col">Role</th>
                            <th scope="col">Site role</th>
                            <th scope="col">Joined</th>
                        </tr>
                    </thead>
                    <tbody>
                        {organisation.members.map(member => (
                            <tr key={member.user_id}>
                                <td>
                                    <UserName userId={member.user_id} name={member.display_name} />
                                </td>
                                <td>
                                    <bdi>{member.email}</bdi>
                                </td>
                                <td>{ORG_ROLE_LABELS[member.role]}</td>
                                <td>{staffRoleLabel(member.site_role)}</td>
                                <td>{formatDate(member.joined_at)}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}
        </Panel>
    );
}

export function AdminOrganisationPage() {
    const { orgId = "" } = useParams();
    const record = useAdminOrganisation(orgId);
    const organisation = record.data;

    const failure = record.error !== null && <LoadFailure message={record.error.message} onRetry={record.refetch} />;
    const back = (
        <p>
            <TextLink to="/admin/organisations">Back to organisations</TextLink>
        </p>
    );

    if (organisation === undefined) {
        return (
            <AdminPage title="Organisation">
                {back}
                {failure}
                {record.error === null && <p className="loading">Loading organisation...</p>}
            </AdminPage>
        );
    }

    return (
        <AdminPage title={organisation.name} description="This organisation's owner, teams and members. View only.">
            {back}
            {failure}
            <div className={styles.sections} key={organisation.id}>
                <OrganisationDetails organisation={organisation} />
                <OrganisationTeams organisation={organisation} />
                <OrganisationMembers organisation={organisation} />
            </div>
        </AdminPage>
    );
}
