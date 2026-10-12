import { useMutation, useQueryClient } from "@tanstack/react-query";
import { deleteSavedTemplate, saveBoardAsTemplate } from "../../api/endpoints/savedTemplates";
import { queryKeys } from "../../api/queryKeys";
import type { SaveTemplateRequest } from "../../types/api";
import type { OrganisationTarget } from "./organisations";
import { refreshWhenChanged } from "./refresh";

export interface BoardTemplateSave extends OrganisationTarget, SaveTemplateRequest {
    boardId: string;
}

const PAYMENT_REQUIRED_STATUS = 402;

const GONE_STATUS = 404;

export function useSaveBoardAsTemplate() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: ({ boardId, name, description, scope }: BoardTemplateSave) =>
            saveBoardAsTemplate(boardId, { name, description, scope }),
        onSettled: (_data, error, { orgId }) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.organisations.savedTemplates(orgId), queryKeys.organisations.entitlements(orgId)],
                [PAYMENT_REQUIRED_STATUS],
            ),
    });
}

export function useDeleteSavedTemplate(orgId: string) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: (templateId: string) => deleteSavedTemplate(orgId, templateId),
        onSettled: (_data, error) =>
            refreshWhenChanged(
                qc,
                error,
                [queryKeys.organisations.savedTemplates(orgId), queryKeys.organisations.entitlements(orgId)],
                [GONE_STATUS],
            ),
    });
}
