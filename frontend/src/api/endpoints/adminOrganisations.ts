import type {
    AdminOrganisationDetailResponse,
    AdminOrganisationListQuery,
    AdminOrganisationListResponse,
} from "../../types/api";
import { apiFetch, buildQueryString } from "../client";

export async function getAdminOrganisations(query: AdminOrganisationListQuery): Promise<AdminOrganisationListResponse> {
    const qs = buildQueryString({ search: query.search, limit: query.limit, offset: query.offset });

    return apiFetch<AdminOrganisationListResponse>(`/admin/organisations${qs}`);
}

export async function getAdminOrganisation(orgId: string): Promise<AdminOrganisationDetailResponse> {
    return apiFetch<AdminOrganisationDetailResponse>(`/admin/organisations/${encodeURIComponent(orgId)}`);
}
