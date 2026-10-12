import type { OrgRole, TeamResponse } from "../../types/api";
import { orgRoleGrants, teamRoleGrants } from "../organisationRoles";
import { isArchived } from "../organisations";
import { NEW_BOARD_NO_TEAM } from "./paths";

export type NewBoardStep = "team" | "template" | "details";

export const NO_TEAM = "";

export const NEW_BOARD_STEP_LABELS: Readonly<Record<NewBoardStep, string>> = {
    team: "Team",
    template: "Template",
    details: "Details",
};

const EVERY_STEP: readonly NewBoardStep[] = ["team", "template", "details"];

const WITHOUT_TEAM_STEP: readonly NewBoardStep[] = ["template", "details"];

export function creatableTeams(teams: readonly TeamResponse[], orgRole: OrgRole | null): TeamResponse[] {
    return teams.filter(team => !isArchived(team) && teamRoleGrants(orgRole, team.role, "team.create_board"));
}

export function createsOrganisationBoards(orgRole: OrgRole | null): boolean {
    return orgRoleGrants(orgRole, "org.create_board");
}

export function presetTeam(creatable: readonly TeamResponse[], requestedId: string | null): TeamResponse | null {
    return creatable.find(team => team.id === requestedId) ?? null;
}

export function presetsNoTeam(requestedId: string | null, organisationBoards: boolean): boolean {
    return organisationBoards && requestedId === NEW_BOARD_NO_TEAM;
}

export function initialTeamChoice(
    creatable: readonly TeamResponse[],
    preset: TeamResponse | null,
    organisationBoards: boolean,
): string {
    if (preset !== null) {
        return preset.id;
    }

    if (organisationBoards) {
        return NO_TEAM;
    }

    return creatable[0]?.id ?? NO_TEAM;
}

export function newBoardSteps(teamPreset: boolean): readonly NewBoardStep[] {
    return teamPreset ? WITHOUT_TEAM_STEP : EVERY_STEP;
}
