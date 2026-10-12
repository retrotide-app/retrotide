import { useSearchParams } from "react-router";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import {
    creatableTeams,
    createsOrganisationBoards,
    NO_TEAM,
    presetsNoTeam,
    presetTeam,
} from "../../domain/board/newBoard";
import { NEW_BOARD_NO_TEAM, NEW_BOARD_TEAM_PARAM } from "../../domain/board/paths";
import { isDeleted } from "../../domain/organisations";
import { SAVED_TEMPLATE_PARAM } from "../../domain/savedTemplates";
import { useTeams } from "../../hooks/queries/teams";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import type { OrganisationResponse, OrgRole } from "../../types/api";
import { DeletedOrganisation } from "../org/shared/DeletedOrganisation";
import { OrgPage } from "../org/shared/OrgPage";
import { NewBoardWizard } from "./NewBoardWizard";

const PAGE_TITLE = "New board";

interface NewBoardProps {
    organisation: OrganisationResponse;
    orgRole: OrgRole | null;
}

function NewBoard({ organisation, orgRole }: NewBoardProps) {
    const [searchParams] = useSearchParams();
    const teams = useTeams(organisation.id);

    if (teams.data === undefined) {
        return (
            <OrgPage title={PAGE_TITLE}>
                {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}
                {teams.error === null && <p className="loading">Loading teams...</p>}
            </OrgPage>
        );
    }

    const creatable = creatableTeams(teams.data.teams, orgRole);
    const organisationBoards = createsOrganisationBoards(orgRole);

    if (creatable.length === 0 && !organisationBoards) {
        return (
            <OrgPage title={PAGE_TITLE}>
                <p>Your role in this organisation does not let you create boards.</p>
            </OrgPage>
        );
    }

    const requested = searchParams.get(NEW_BOARD_TEAM_PARAM);
    const preset = presetTeam(creatable, requested);
    const noTeamPreset = preset === null && presetsNoTeam(requested, organisationBoards);
    const presetTemplate = searchParams.get(SAVED_TEMPLATE_PARAM);

    return (
        <OrgPage title={PAGE_TITLE} description="Choose who the board is for and a template, then give it a name.">
            {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}
            <NewBoardWizard
                key={`${preset?.id ?? (noTeamPreset ? NEW_BOARD_NO_TEAM : NO_TEAM)}:${presetTemplate ?? ""}`}
                organisation={organisation}
                teams={creatable}
                preset={preset}
                noTeamPreset={noTeamPreset}
                organisationBoards={organisationBoards}
                presetTemplate={presetTemplate}
            />
        </OrgPage>
    );
}

export function NewBoardPage() {
    const { current, role } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return <NewBoard key={current.id} organisation={current} orgRole={role} />;
}
