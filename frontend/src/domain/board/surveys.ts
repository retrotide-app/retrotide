import type {
    SurveyAnswer,
    SurveyQuestionKind,
    SurveyQuestionRequest,
    SurveyQuestionView,
    SurveyTemplate,
    SurveyTemplateCategory,
    SurveyView,
} from "../../types/api";
import { errorCode } from "../../utils/errorMessage";

export interface QuestionDraft {
    kind: SurveyQuestionKind;
    prompt: string;
    options: string[];
    scale_min: number;
    scale_max: number;
}

export interface SurveyDraft {
    title: string;
    questions: QuestionDraft[];
}

export interface SurveyRequestDraft {
    title: string;
    questions: Omit<SurveyQuestionRequest, "id">[];
}

export interface ResultRow {
    label: string;
    count: number;
    share: number;
}

export interface TemplateGroup {
    category: SurveyTemplateCategory;
    label: string;
    templates: SurveyTemplate[];
}

export interface QuestionKindOption {
    value: SurveyQuestionKind;
    label: string;
}

export const SURVEY_TITLE_MAX_LENGTH = 120;

export const SURVEY_PROMPT_MAX_LENGTH = 300;

export const SURVEY_OPTION_MAX_LENGTH = 100;

export const SURVEY_MAX_QUESTIONS = 20;

export const SURVEY_MIN_OPTIONS = 2;

export const SURVEY_MAX_OPTIONS = 10;

export const SURVEY_SCALE_FLOOR = 0;

export const SURVEY_SCALE_CEILING = 10;

const DEFAULT_SCALE_MIN = 1;

const DEFAULT_SCALE_MAX = 5;

const PERCENT = 100;

export const QUESTION_KINDS: readonly QuestionKindOption[] = [
    { value: "single_choice", label: "Single choice" },
    { value: "multiple_choice", label: "Multiple choice" },
    { value: "scale", label: "Scale" },
];

export const SURVEY_CATEGORIES: readonly SurveyTemplateCategory[] = ["health_check", "survey"];

export const SURVEY_CATEGORY_LABELS: Readonly<Record<SurveyTemplateCategory, string>> = {
    health_check: "Health check",
    survey: "Survey",
};

const SURVEY_REFUSALS: Readonly<Record<string, string>> = {
    survey_title_required: "Give the survey a title.",
    survey_title_too_long: `A survey title can be at most ${SURVEY_TITLE_MAX_LENGTH} characters.`,
    survey_questions_required: "Add at least one question.",
    too_many_questions: `A survey can have at most ${SURVEY_MAX_QUESTIONS} questions.`,
    invalid_question_kind: "Each question must be single choice, multiple choice or a scale.",
    question_prompt_required: "Every question needs a prompt.",
    question_prompt_too_long: `A question's prompt can be at most ${SURVEY_PROMPT_MAX_LENGTH} characters.`,
    invalid_options: `A choice question needs ${SURVEY_MIN_OPTIONS} to ${SURVEY_MAX_OPTIONS} different options of at most ${SURVEY_OPTION_MAX_LENGTH} characters each.`,
    invalid_scale: `A scale must run from a lower to a higher whole number between ${SURVEY_SCALE_FLOOR} and ${SURVEY_SCALE_CEILING}.`,
    unknown_survey_template: "That survey template is no longer offered.",
    template_with_questions: "A survey comes from a template or from your own questions, not both.",
    answers_required: "Answer at least one question.",
    invalid_answer: "One of your answers does not fit its question.",
    unknown_question: "One of your answers is to a question this survey does not have.",
    survey_closed: "This survey has closed, so it takes no more answers.",
    too_many_surveys: "A board can hold at most 20 surveys.",
};

function characters(text: string): number {
    return [...text].length;
}

export function newQuestionDraft(kind: SurveyQuestionKind): QuestionDraft {
    const options = kind === "scale" ? [] : ["", ""];

    return { kind, prompt: "", options, scale_min: DEFAULT_SCALE_MIN, scale_max: DEFAULT_SCALE_MAX };
}

export function withKind(question: QuestionDraft, kind: SurveyQuestionKind): QuestionDraft {
    if (kind !== "scale" && question.kind !== "scale") {
        return { ...question, kind };
    }

    return { ...newQuestionDraft(kind), prompt: question.prompt };
}

export function emptySurveyDraft(): SurveyDraft {
    return { title: "", questions: [newQuestionDraft("single_choice")] };
}

export function draftFromTemplate(template: SurveyTemplate): SurveyDraft {
    return {
        title: template.title,
        questions: template.questions.map(question => ({
            kind: question.kind,
            prompt: question.prompt,
            options: [...question.options],
            scale_min: question.scale_min ?? DEFAULT_SCALE_MIN,
            scale_max: question.scale_max ?? DEFAULT_SCALE_MAX,
        })),
    };
}

function optionsRefusal(position: number, options: readonly string[]): string | null {
    const trimmed = options.map(option => option.trim());

    if (trimmed.some(option => option === "")) {
        return `Question ${position} has an empty option.`;
    }

    if (trimmed.length < SURVEY_MIN_OPTIONS) {
        return `Question ${position} needs at least ${SURVEY_MIN_OPTIONS} options.`;
    }

    if (trimmed.length > SURVEY_MAX_OPTIONS) {
        return `Question ${position} can have at most ${SURVEY_MAX_OPTIONS} options.`;
    }

    if (trimmed.some(option => characters(option) > SURVEY_OPTION_MAX_LENGTH)) {
        return `Each option of question ${position} can be at most ${SURVEY_OPTION_MAX_LENGTH} characters.`;
    }

    const folded = new Set(trimmed.map(option => option.toLowerCase()));
    if (folded.size !== trimmed.length) {
        return `Question ${position} lists the same option twice.`;
    }

    return null;
}

function scaleRefusal(position: number, low: number, high: number): string | null {
    const whole = Number.isInteger(low) && Number.isInteger(high);

    if (!whole || low < SURVEY_SCALE_FLOOR || high > SURVEY_SCALE_CEILING || low >= high) {
        return `Question ${position}'s scale must run from a lower to a higher whole number between ${SURVEY_SCALE_FLOOR} and ${SURVEY_SCALE_CEILING}.`;
    }

    return null;
}

function questionRefusal(question: QuestionDraft, position: number): string | null {
    const prompt = question.prompt.trim();

    if (prompt === "") {
        return `Question ${position} needs a prompt.`;
    }

    if (characters(prompt) > SURVEY_PROMPT_MAX_LENGTH) {
        return `Question ${position}'s prompt can be at most ${SURVEY_PROMPT_MAX_LENGTH} characters.`;
    }

    if (question.kind === "scale") {
        return scaleRefusal(position, question.scale_min, question.scale_max);
    }

    return optionsRefusal(position, question.options);
}

export function validateDraft(draft: SurveyDraft): string | null {
    const title = draft.title.trim();

    if (title === "") {
        return "Give the survey a title.";
    }

    if (characters(title) > SURVEY_TITLE_MAX_LENGTH) {
        return `A survey title can be at most ${SURVEY_TITLE_MAX_LENGTH} characters.`;
    }

    if (draft.questions.length === 0) {
        return "Add at least one question.";
    }

    if (draft.questions.length > SURVEY_MAX_QUESTIONS) {
        return `A survey can have at most ${SURVEY_MAX_QUESTIONS} questions.`;
    }

    for (const [index, question] of draft.questions.entries()) {
        const refusal = questionRefusal(question, index + 1);
        if (refusal !== null) {
            return refusal;
        }
    }

    return null;
}

export function draftRequest(draft: SurveyDraft): SurveyRequestDraft {
    return {
        title: draft.title.trim(),
        questions: draft.questions.map(question => {
            const prompt = question.prompt.trim();

            if (question.kind === "scale") {
                return {
                    kind: question.kind,
                    prompt,
                    options: [],
                    scale_min: question.scale_min,
                    scale_max: question.scale_max,
                };
            }

            return {
                kind: question.kind,
                prompt,
                options: question.options.map(option => option.trim()),
                scale_min: null,
                scale_max: null,
            };
        }),
    };
}

export function scaleValues(question: Pick<SurveyQuestionView, "scale_min" | "scale_max">): number[] {
    const { scale_min: low, scale_max: high } = question;

    if (low === null || high === null || high < low) {
        return [];
    }

    return Array.from({ length: high - low + 1 }, (_, offset) => low + offset);
}

export function pick(question: SurveyQuestionView, current: readonly number[], value: number): number[] {
    if (question.kind !== "multiple_choice") {
        return [value];
    }

    if (current.includes(value)) {
        return current.filter(picked => picked !== value);
    }

    return [...current, value].toSorted((left, right) => left - right);
}

export function answersComplete(survey: SurveyView, picks: Readonly<Record<string, readonly number[]>>): boolean {
    return survey.questions.every(question => (picks[question.id] ?? []).length > 0);
}

export function surveyAnswers(survey: SurveyView, picks: Readonly<Record<string, readonly number[]>>): SurveyAnswer[] {
    return survey.questions
        .filter(question => (picks[question.id] ?? []).length > 0)
        .map(question => ({ question_id: question.id, answer: [...(picks[question.id] ?? [])] }));
}

export function resultRows(question: SurveyQuestionView): ResultRow[] {
    const results = question.results;

    if (results === null) {
        return [];
    }

    const labels = question.kind === "scale" ? scaleValues(question).map(String) : question.options;

    return labels.map((label, index) => {
        const count = results.counts[index] ?? 0;
        const share = results.responses > 0 ? count / results.responses : 0;

        return { label, count, share };
    });
}

export function sharePercent(row: ResultRow): number {
    return Math.round(Math.min(1, Math.max(0, row.share)) * PERCENT);
}

export function formatAverage(average: number | null): string {
    if (average === null) {
        return "No scores yet";
    }

    return average.toLocaleString("en-GB", { maximumFractionDigits: 2 });
}

export function respondentsLabel(count: number): string {
    return count === 1 ? "1 person answered" : `${count} people answered`;
}

export function surveyRefusal(error: Error): string {
    const code = errorCode(error);

    if (code !== null && Object.hasOwn(SURVEY_REFUSALS, code)) {
        return SURVEY_REFUSALS[code];
    }

    return error.message;
}

export function templatesByCategory(templates: readonly SurveyTemplate[]): TemplateGroup[] {
    return SURVEY_CATEGORIES.map(category => ({
        category,
        label: SURVEY_CATEGORY_LABELS[category],
        templates: templates.filter(template => template.category === category),
    })).filter(group => group.templates.length > 0);
}
