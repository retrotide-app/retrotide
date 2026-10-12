import { useQuery } from "@tanstack/react-query";
import { listSavedTemplates } from "../../api/endpoints/savedTemplates";
import { queryKeys } from "../../api/queryKeys";
import type { SavedTemplateListResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useSavedTemplates(orgId: string): QueryState<SavedTemplateListResponse> {
    const templates = useQuery({
        queryKey: queryKeys.organisations.savedTemplates(orgId),
        queryFn: () => listSavedTemplates(orgId),
        enabled: orgId !== "",
    });

    return queryState(templates);
}
