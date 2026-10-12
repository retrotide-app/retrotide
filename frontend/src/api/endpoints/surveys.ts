import type {
    BoardCommandRequest,
    BoardCommandResponse,
    CreateSurveyRequest,
    SurveyResponseRequest,
    SurveyTemplateListResponse,
} from "../../types/api";
import { apiDeleteWithBody, apiFetch, apiPost, apiPut } from "../client";

function surveysPath(boardId: string, rest = ""): string {
    return `/boards/${encodeURIComponent(boardId)}/surveys${rest}`;
}

function surveyPath(boardId: string, surveyId: string, action = ""): string {
    return surveysPath(boardId, `/${encodeURIComponent(surveyId)}${action}`);
}

export async function listSurveyTemplates(): Promise<SurveyTemplateListResponse> {
    return apiFetch<SurveyTemplateListResponse>("/survey-templates");
}

export async function createSurvey(boardId: string, body: CreateSurveyRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CreateSurveyRequest>(surveysPath(boardId), body);
}

export async function closeSurvey(
    boardId: string,
    surveyId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(surveyPath(boardId, surveyId, "/close"), body);
}

export async function deleteSurvey(
    boardId: string,
    surveyId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(surveyPath(boardId, surveyId), body);
}

export async function respondToSurvey(
    boardId: string,
    surveyId: string,
    body: SurveyResponseRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, SurveyResponseRequest>(surveyPath(boardId, surveyId, "/responses"), body);
}
