import { useQuery } from "@tanstack/react-query";
import { listSurveyTemplates } from "../../api/endpoints/surveys";
import { queryKeys } from "../../api/queryKeys";
import type { SurveyTemplateListResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

const TEMPLATES_STALE_MS = 60 * 60 * 1000;

export function useSurveyTemplates(enabled = true): QueryState<SurveyTemplateListResponse> {
    const templates = useQuery({
        queryKey: queryKeys.surveyTemplates(),
        queryFn: listSurveyTemplates,
        staleTime: TEMPLATES_STALE_MS,
        enabled,
    });

    return queryState(templates);
}
