import type { Colour } from "./board";

export interface TemplateColumnResponse {
    title: string;
    description: string;
    colour: Colour;
}

export interface TemplateResponse {
    key: string;
    name: string;
    description: string;
    columns: TemplateColumnResponse[];
}

export interface TemplateListResponse {
    templates: TemplateResponse[];
}
