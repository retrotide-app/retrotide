import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { getAdminOrganisation, getAdminOrganisations } from "../../api/endpoints/adminOrganisations";
import { queryKeys } from "../../api/queryKeys";
import type {
    AdminOrganisationDetailResponse,
    AdminOrganisationListQuery,
    AdminOrganisationListResponse,
} from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useAdminOrganisations(query: AdminOrganisationListQuery): QueryState<AdminOrganisationListResponse> {
    const organisations = useQuery({
        queryKey: queryKeys.admin.organisations.list(query),
        queryFn: () => getAdminOrganisations(query),
        placeholderData: keepPreviousData,
    });

    return queryState(organisations);
}

export function useAdminOrganisation(orgId: string): QueryState<AdminOrganisationDetailResponse> {
    const organisation = useQuery({
        queryKey: queryKeys.admin.organisations.detail(orgId),
        queryFn: () => getAdminOrganisation(orgId),
        enabled: orgId !== "",
    });

    return queryState(organisation);
}
