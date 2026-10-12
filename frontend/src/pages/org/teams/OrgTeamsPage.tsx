import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Toast } from "../../../components/Toast/Toast";
import { boardCountsByTeam } from "../../../domain/board/teamBoardCounts";
import { orgRoleGrants } from "../../../domain/organisationRoles";
import { isDeleted } from "../../../domain/organisations";
import { useCreateTeam } from "../../../hooks/mutations/teams";
import { useOrganisationBoards } from "../../../hooks/queries/boards";
import { useTeams } from "../../../hooks/queries/teams";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import type { OrganisationResponse, OrgRole } from "../../../types/api";
import { DeletedOrganisation } from "../shared/DeletedOrganisation";
import { OrgPage } from "../shared/OrgPage";
import { TeamNameDialog } from "../shared/TeamNameDialog";
import styles from "./OrgTeamsPage.module.css";
import { TeamCard } from "./TeamCard";

const PAGE_TITLE = "Teams";

interface TeamGridProps {
    organisation: OrganisationResponse;
    orgRole: OrgRole | null;
}

function TeamGrid({ organisation, orgRole }: TeamGridProps) {
    const teams = useTeams(organisation.id);
    const boards = useOrganisationBoards(organisation.id);

    const listed = teams.data?.teams ?? [];
    const counts = boards.data === undefined ? null : boardCountsByTeam(boards.data.boards);

    return (
        <>
            {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}

            {boards.error !== null && <LoadFailure message={boards.error.message} onRetry={boards.refetch} />}

            {teams.data === undefined && teams.error === null && <p className="loading">Loading teams...</p>}

            {teams.data !== undefined && listed.length === 0 && (
                <p>
                    {orgRoleGrants(orgRole, "org.manage_teams")
                        ? "No teams yet."
                        : "You're not on any teams yet. An owner or admin can add you to one."}
                </p>
            )}

            {listed.length > 0 && (
                <ul className={styles.grid} aria-label={`Teams in ${organisation.name}`}>
                    {listed.map(team => (
                        <li key={team.id}>
                            <TeamCard
                                organisation={organisation}
                                team={team}
                                orgRole={orgRole}
                                boardCount={counts === null ? null : (counts.get(team.id) ?? 0)}
                            />
                        </li>
                    ))}
                </ul>
            )}
        </>
    );
}

interface NewTeamButtonProps {
    organisation: OrganisationResponse;
}

function NewTeamButton({ organisation }: NewTeamButtonProps) {
    const creation = useCreateTeam();
    const [open, setOpen] = useState(false);

    const created = creation.data;

    function openDialog() {
        creation.reset();
        setOpen(true);
    }

    function handleCreate(name: string) {
        creation.mutate({ orgId: organisation.id, name }, { onSuccess: () => setOpen(false) });
    }

    return (
        <>
            <Button type="button" variant="primary" onClick={openDialog}>
                New team
            </Button>

            <TeamNameDialog
                open={open}
                title="New team"
                submitLabel="Create team"
                busy={creation.isPending}
                error={creation.error}
                onSubmit={handleCreate}
                onClose={() => setOpen(false)}
            />

            {creation.isSuccess && created !== undefined && (
                <Toast variant="success" onDismiss={creation.reset}>
                    <bdi>{created.name}</bdi> created.
                </Toast>
            )}
        </>
    );
}

export function OrgTeamsPage() {
    const { current, role } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    const manages = orgRoleGrants(role, "org.manage_teams");

    const header = manages && (
        <div className={styles.headerActions}>
            <NewTeamButton key={current.id} organisation={current} />
        </div>
    );

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="A team is a group of people in your organisation. Boards made for a team are shared with everyone on it."
            status={header}
        >
            <TeamGrid key={current.id} organisation={current} orgRole={role} />
        </OrgPage>
    );
}
