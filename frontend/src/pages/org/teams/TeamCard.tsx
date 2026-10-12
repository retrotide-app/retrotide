import { useId } from "react";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { BOARD_NOUN, MEMBER_NOUN } from "../../../domain/board/organisationBoards";
import { effectiveTeamRole, TEAM_ROLE_LABELS, teamRoleGrants } from "../../../domain/organisationRoles";
import { isArchived, teamPath } from "../../../domain/organisations";
import { useTeamMembers } from "../../../hooks/queries/teams";
import type { OrganisationResponse, OrgRole, TeamResponse } from "../../../types/api";
import { countOf } from "../../../utils/chart";
import styles from "./OrgTeamsPage.module.css";

interface TeamCardProps {
    organisation: OrganisationResponse;
    team: TeamResponse;
    orgRole: OrgRole | null;
    boardCount: number | null;
}

function roleLabel(orgRole: OrgRole | null, team: TeamResponse): string {
    const role = orgRole === null ? null : effectiveTeamRole(orgRole, team.role);

    if (role === null) {
        return "Not on this team";
    }

    return TEAM_ROLE_LABELS[role];
}

export function TeamCard({ organisation, team, orgRole, boardCount }: TeamCardProps) {
    const titleId = useId();
    const viewable = teamRoleGrants(orgRole, team.role, "team.view");
    const members = useTeamMembers(viewable ? team.id : "");

    const memberCount = members.data?.members.length;

    return (
        <article aria-labelledby={titleId} className={styles.card}>
            <div className={styles.cardHeader}>
                <h2 id={titleId} className={styles.cardTitle} dir="auto">
                    {team.name}
                </h2>
                {isArchived(team) && <StatusTag tone="neutral">Archived</StatusTag>}
            </div>

            <dl className={styles.facts}>
                <div className={styles.fact}>
                    <dt>Your role</dt>
                    <dd>{roleLabel(orgRole, team)}</dd>
                </div>
                {memberCount !== undefined && (
                    <div className={styles.fact}>
                        <dt>Members</dt>
                        <dd>{countOf(memberCount, MEMBER_NOUN)}</dd>
                    </div>
                )}
                {viewable && boardCount !== null && (
                    <div className={styles.fact}>
                        <dt>Boards</dt>
                        <dd>{countOf(boardCount, BOARD_NOUN)}</dd>
                    </div>
                )}
            </dl>

            {members.error !== null && <LoadFailure message={members.error.message} onRetry={members.refetch} />}

            {viewable && (
                <div className={styles.cardActions}>
                    <ButtonLink to={teamPath(organisation.slug, team.id)} size="small" aria-describedby={titleId}>
                        Open
                    </ButtonLink>
                </div>
            )}
        </article>
    );
}
