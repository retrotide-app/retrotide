import { useQuery } from "@tanstack/react-query";
import {
    getEntitlements,
    getOrganisation,
    listOrganisationMembers,
    listOrganisations,
} from "../../api/endpoints/organisations";
import { queryKeys } from "../../api/queryKeys";
import type {
    EntitlementsResponse,
    OrganisationListResponse,
    OrganisationMemberListResponse,
    OrganisationResponse,
} from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useOrganisations(enabled = true): QueryState<OrganisationListResponse> {
    const organisations = useQuery({
        queryKey: queryKeys.organisations.list(),
        queryFn: listOrganisations,
        enabled,
    });

    return queryState(organisations);
}

export function useOrganisation(orgId: string): QueryState<OrganisationResponse> {
    const organisation = useQuery({
        queryKey: queryKeys.organisations.detail(orgId),
        queryFn: () => getOrganisation(orgId),
        enabled: orgId !== "",
    });

    return queryState(organisation);
}

export function useOrganisationMembers(orgId: string): QueryState<OrganisationMemberListResponse> {
    const members = useQuery({
        queryKey: queryKeys.organisations.members(orgId),
        queryFn: () => listOrganisationMembers(orgId),
        enabled: orgId !== "",
    });

    return queryState(members);
}

export function useEntitlements(orgId: string): QueryState<EntitlementsResponse> {
    const entitlements = useQuery({
        queryKey: queryKeys.organisations.entitlements(orgId),
        queryFn: () => getEntitlements(orgId),
        enabled: orgId !== "",
    });

    return queryState(entitlements);
}
