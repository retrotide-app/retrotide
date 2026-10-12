import { closeSurvey, createSurvey, deleteSurvey, respondToSurvey } from "../../api/endpoints/surveys";
import type { SurveyAnswer, SurveyQuestionRequest } from "../../types/api";
import { useBoardCommand } from "./boardCommand";
import { newId } from "./ids";

export interface SurveyCreation {
    title: string;
    template_key?: string;
    questions?: Omit<SurveyQuestionRequest, "id">[];
}

export interface SurveyTarget {
    surveyId: string;
}

export interface SurveyAnswers extends SurveyTarget {
    answers: SurveyAnswer[];
}

export function useCreateSurvey(boardId: string) {
    return useBoardCommand(boardId, ({ title, template_key, questions }: SurveyCreation, opId) =>
        createSurvey(boardId, {
            op_id: opId,
            id: newId(),
            title,
            template_key,
            questions: questions?.map(question => ({ ...question, id: newId() })),
        }),
    );
}

export function useCloseSurvey(boardId: string) {
    return useBoardCommand(boardId, ({ surveyId }: SurveyTarget, opId) =>
        closeSurvey(boardId, surveyId, { op_id: opId }),
    );
}

export function useDeleteSurvey(boardId: string) {
    return useBoardCommand(boardId, ({ surveyId }: SurveyTarget, opId) =>
        deleteSurvey(boardId, surveyId, { op_id: opId }),
    );
}

export function useRespondToSurvey(boardId: string) {
    return useBoardCommand(boardId, ({ surveyId, answers }: SurveyAnswers, opId) =>
        respondToSurvey(boardId, surveyId, { op_id: opId, answers }),
    );
}
