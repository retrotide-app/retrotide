import { useQuery } from "@tanstack/react-query";
import { getTeam, listTeamMembers, listTeams } from "../../api/endpoints/teams";
import { queryKeys } from "../../api/queryKeys";
import type { TeamListResponse, TeamMemberListResponse, TeamResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useTeams(orgId: string): QueryState<TeamListResponse> {
    const teams = useQuery({
        queryKey: queryKeys.organisations.teams(orgId),
        queryFn: () => listTeams(orgId),
        enabled: orgId !== "",
    });

    return queryState(teams);
}

export function useTeam(teamId: string): QueryState<TeamResponse> {
    const team = useQuery({
        queryKey: queryKeys.teams.detail(teamId),
        queryFn: () => getTeam(teamId),
        enabled: teamId !== "",
    });

    return queryState(team);
}

export function useTeamMembers(teamId: string): QueryState<TeamMemberListResponse> {
    const members = useQuery({
        queryKey: queryKeys.teams.members(teamId),
        queryFn: () => listTeamMembers(teamId),
        enabled: teamId !== "",
    });

    return queryState(members);
}
