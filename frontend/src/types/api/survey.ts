import type { BoardCommandRequest } from "./board";

export type SurveyQuestionKind = "single_choice" | "multiple_choice" | "scale";

export type SurveyStatus = "open" | "closed";

export interface SurveyResultsView {
    counts: number[];
    responses: number;
    average: number | null;
}

export interface SurveyQuestionView {
    id: string;
    kind: SurveyQuestionKind;
    prompt: string;
    options: string[];
    scale_min: number | null;
    scale_max: number | null;
    results: SurveyResultsView | null;
}

export interface SurveyView {
    id: string;
    title: string;
    template_key: string;
    status: SurveyStatus;
    questions: SurveyQuestionView[];
    respondents: number;
    my_answers: Record<string, number[]>;
}

export interface SurveyQuestionRequest {
    id: string;
    kind: SurveyQuestionKind;
    prompt: string;
    options?: string[];
    scale_min?: number | null;
    scale_max?: number | null;
}

export interface CreateSurveyRequest extends BoardCommandRequest {
    id: string;
    title: string;
    template_key?: string;
    questions?: SurveyQuestionRequest[];
}

export interface SurveyAnswer {
    question_id: string;
    answer: number[];
}

export interface SurveyResponseRequest extends BoardCommandRequest {
    answers: SurveyAnswer[];
}

export type SurveyTemplateCategory = "health_check" | "survey";

export interface SurveyTemplateQuestion {
    kind: SurveyQuestionKind;
    prompt: string;
    options: string[];
    scale_min: number | null;
    scale_max: number | null;
}

export interface SurveyTemplate {
    key: string;
    title: string;
    description: string;
    category: SurveyTemplateCategory;
    questions: SurveyTemplateQuestion[];
}

export interface SurveyTemplateListResponse {
    templates: SurveyTemplate[];
}
