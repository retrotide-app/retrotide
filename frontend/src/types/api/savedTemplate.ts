import type { TemplateColumnResponse } from "./boardTemplate";

export type SavedTemplateScope = "personal" | "team";

export interface SavedTemplateResponse {
    id: string;
    scope: SavedTemplateScope;
    team_id?: string;
    team_name?: string;
    name: string;
    description: string;
    columns: TemplateColumnResponse[];
    created_at: string;
    can_delete: boolean;
}

export interface SavedTemplateListResponse {
    templates: SavedTemplateResponse[];
}

export interface SaveTemplateRequest {
    name: string;
    description: string;
    scope: SavedTemplateScope;
}
