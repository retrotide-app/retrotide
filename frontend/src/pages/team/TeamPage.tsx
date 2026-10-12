import { useParams } from "react-router";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../components/StatusTag/StatusTag";
import { TextLink } from "../../components/TextLink/TextLink";
import { newBoardPath } from "../../domain/board/paths";
import { orgRoleGrants, teamRoleGrants, type TeamViewer } from "../../domain/organisationRoles";
import { isArchived, isDeleted, organisationPath } from "../../domain/organisations";
import { useTeam } from "../../hooks/queries/teams";
import { useAuth } from "../../hooks/useAuth";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import type { OrganisationResponse, OrgRole, StaffRole } from "../../types/api";
import { DeletedOrganisation } from "../org/shared/DeletedOrganisation";
import { OrgPage } from "../org/shared/OrgPage";
import { TeamBoardsSection } from "./TeamBoardsSection";
import { TeamMembersSection } from "./TeamMembersSection";
import styles from "./TeamPage.module.css";
import { TeamSettingsMenu } from "./TeamSettingsMenu";

const PAGE_TITLE = "Team";

interface TeamDetailProps {
    organisation: OrganisationResponse;
    teamId: string;
    viewerId: string;
    orgRole: OrgRole | null;
    siteRole: StaffRole | null;
}

function TeamDetail({ organisation, teamId, viewerId, orgRole, siteRole }: TeamDetailProps) {
    const team = useTeam(teamId);
    const found = team.data;

    const back = (
        <p className={styles.back}>
            <TextLink to={organisationPath(organisation.slug, "teams")}>Back to teams</TextLink>
        </p>
    );
    const failure = team.error !== null && <LoadFailure message={team.error.message} onRetry={team.refetch} />;

    if (found === undefined) {
        return (
            <OrgPage title={PAGE_TITLE}>
                {back}
                {failure}
                {team.error === null && <p className="loading">Loading team...</p>}
            </OrgPage>
        );
    }

    if (found.org_id !== organisation.id) {
        return (
            <OrgPage title="Team not found">
                {back}
                <p>
                    This team is not part of <bdi>{organisation.name}</bdi>.
                </p>
            </OrgPage>
        );
    }

    const archived = isArchived(found);
    const viewer: TeamViewer = { userId: viewerId, orgRole, teamRole: found.role ?? null, siteRole };
    const renames = teamRoleGrants(orgRole, found.role, "team.update");
    const managesTeams = orgRoleGrants(orgRole, "org.manage_teams");
    const managesMembers = !archived && teamRoleGrants(orgRole, found.role, "team.manage_members");
    const createsBoards = !archived && teamRoleGrants(orgRole, found.role, "team.create_board");

    const header = (
        <>
            {archived && <StatusTag tone="neutral">Archived</StatusTag>}
            <div className={styles.headerActions}>
                {createsBoards && (
                    <ButtonLink to={newBoardPath(organisation.slug, found.id)} variant="primary">
                        New board
                    </ButtonLink>
                )}
                <TeamSettingsMenu organisation={organisation} team={found} renames={renames} manages={managesTeams} />
            </div>
        </>
    );

    return (
        <OrgPage
            title={found.name}
            description="This team's boards and people. Everyone on the team can open its boards."
            status={header}
        >
            {back}
            {failure}
            {archived && <p>This team is archived, so nobody can rename it, change its members or add boards.</p>}
            <div className={styles.sections}>
                <TeamBoardsSection organisation={organisation} team={found} createsBoards={createsBoards} />
                <TeamMembersSection
                    organisation={organisation}
                    team={found}
                    viewer={viewer}
                    managesMembers={managesMembers}
                />
            </div>
        </OrgPage>
    );
}

export function TeamPage() {
    const { teamId = "" } = useParams();
    const { current, role } = useCurrentOrg();
    const { user, siteRole } = useAuth();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return (
        <TeamDetail
            key={`${current.id}/${teamId}`}
            organisation={current}
            teamId={teamId}
            viewerId={user?.id ?? ""}
            orgRole={role}
            siteRole={siteRole}
        />
    );
}
