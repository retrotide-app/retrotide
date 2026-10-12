import type { TemplateListResponse, TemplateResponse } from "../../types/api";
import { apiFetch } from "../client";

export async function listBoardTemplates(): Promise<TemplateListResponse> {
    return apiFetch<TemplateListResponse>("/templates");
}

export async function getBoardTemplate(key: string): Promise<TemplateResponse> {
    return apiFetch<TemplateResponse>(`/templates/${encodeURIComponent(key)}`);
}
