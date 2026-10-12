import { useQuery } from "@tanstack/react-query";
import { listInvites, lookUpInvite } from "../../api/endpoints/invites";
import { queryKeys } from "../../api/queryKeys";
import type { InviteDetailsResponse, OrganisationInviteListResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useOrganisationInvites(orgId: string, enabled = true): QueryState<OrganisationInviteListResponse> {
    const invites = useQuery({
        queryKey: queryKeys.organisations.invites(orgId),
        queryFn: () => listInvites(orgId),
        enabled: enabled && orgId !== "",
    });

    return queryState(invites);
}

export function useInviteLookup(token: string): QueryState<InviteDetailsResponse> {
    const invite = useQuery({
        queryKey: queryKeys.invites.lookup(token),
        queryFn: () => lookUpInvite({ token }),
        enabled: token !== "",
    });

    return queryState(invite);
}
