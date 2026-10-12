import type { SavedTemplateListResponse, SavedTemplateResponse, SaveTemplateRequest } from "../../types/api";
import { apiDelete, apiFetch, apiPost } from "../client";

function organisationSavedTemplatesPath(orgId: string, action = ""): string {
    return `/orgs/${encodeURIComponent(orgId)}/saved-templates${action}`;
}

export async function listSavedTemplates(orgId: string): Promise<SavedTemplateListResponse> {
    return apiFetch<SavedTemplateListResponse>(organisationSavedTemplatesPath(orgId));
}

export async function saveBoardAsTemplate(boardId: string, body: SaveTemplateRequest): Promise<SavedTemplateResponse> {
    return apiPost<SavedTemplateResponse, SaveTemplateRequest>(
        `/boards/${encodeURIComponent(boardId)}/saved-templates`,
        body,
    );
}

export async function deleteSavedTemplate(orgId: string, templateId: string): Promise<void> {
    return apiDelete<void>(organisationSavedTemplatesPath(orgId, `/${encodeURIComponent(templateId)}`));
}
