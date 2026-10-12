import { describe, expect, it } from "vitest";
import { makeApiError, makeSurvey, makeSurveyQuestion } from "../../test-utils/fixtures";
import type { SurveyTemplate } from "../../types/api";
import {
    answersComplete,
    draftFromTemplate,
    draftRequest,
    emptySurveyDraft,
    formatAverage,
    newQuestionDraft,
    pick,
    type QuestionDraft,
    respondentsLabel,
    resultRows,
    scaleValues,
    sharePercent,
    surveyAnswers,
    surveyRefusal,
    templatesByCategory,
    validateDraft,
    withKind,
} from "./surveys";

const mood = makeSurveyQuestion({
    id: "q-mood",
    kind: "single_choice",
    prompt: "How do you feel?",
    options: ["Good", "Okay", "Bad"],
    scale_min: null,
    scale_max: null,
});

const focus = makeSurveyQuestion({
    id: "q-focus",
    kind: "multiple_choice",
    prompt: "What should we focus on?",
    options: ["Quality", "Speed", "Process"],
    scale_min: null,
    scale_max: null,
});

const pace = makeSurveyQuestion({ id: "q-pace" });

const healthCheck: SurveyTemplate = {
    key: "health_check",
    title: "Team health check",
    description: "Eight quick ratings.",
    category: "health_check",
    questions: [{ kind: "scale", prompt: "Fun?", options: [], scale_min: 1, scale_max: 5 }],
};

const teamMood: SurveyTemplate = {
    key: "team_mood",
    title: "Team mood",
    description: "A single pick.",
    category: "survey",
    questions: [
        { kind: "single_choice", prompt: "Mood?", options: ["Great", "Bad"], scale_min: null, scale_max: null },
    ],
};

function choice(overrides: Partial<QuestionDraft> = {}): QuestionDraft {
    return { ...newQuestionDraft("single_choice"), prompt: "Mood?", options: ["Good", "Bad"], ...overrides };
}

function scale(overrides: Partial<QuestionDraft> = {}): QuestionDraft {
    return { ...newQuestionDraft("scale"), prompt: "Pace?", ...overrides };
}

describe("newQuestionDraft", () => {
    it("starts a choice question with two empty options and a scale on one to five", () => {
        // when
        const single = newQuestionDraft("single_choice");
        const rating = newQuestionDraft("scale");

        // then
        expect(single).toEqual({ kind: "single_choice", prompt: "", options: ["", ""], scale_min: 1, scale_max: 5 });
        expect(rating).toEqual({ kind: "scale", prompt: "", options: [], scale_min: 1, scale_max: 5 });
    });

    it("keeps the options when a choice question changes between single and multiple", () => {
        // when
        const multiple = withKind(choice({ options: ["Good", "Bad", "Ugly"] }), "multiple_choice");

        // then
        expect(multiple).toEqual({ ...choice({ options: ["Good", "Bad", "Ugly"] }), kind: "multiple_choice" });
    });

    it("keeps only the prompt when a question turns into or out of a scale", () => {
        // when
        const rating = withKind(choice({ prompt: "Mood?" }), "scale");
        const single = withKind(scale({ prompt: "Pace?", scale_min: 0, scale_max: 10 }), "single_choice");

        // then
        expect(rating).toEqual({ kind: "scale", prompt: "Mood?", options: [], scale_min: 1, scale_max: 5 });
        expect(single).toEqual({
            kind: "single_choice",
            prompt: "Pace?",
            options: ["", ""],
            scale_min: 1,
            scale_max: 5,
        });
    });

    it("starts a custom survey with one question", () => {
        // when
        const draft = emptySurveyDraft();

        // then
        expect(draft.title).toBe("");
        expect(draft.questions).toHaveLength(1);
    });
});

describe("draftFromTemplate", () => {
    it("copies the template's title and questions", () => {
        // when
        const draft = draftFromTemplate(teamMood);

        // then
        expect(draft).toEqual({
            title: "Team mood",
            questions: [
                { kind: "single_choice", prompt: "Mood?", options: ["Great", "Bad"], scale_min: 1, scale_max: 5 },
            ],
        });
    });

    it("never shares the template's options", () => {
        // given
        const draft = draftFromTemplate(teamMood);

        // when
        draft.questions[0].options[0] = "Changed";

        // then
        expect(teamMood.questions[0].options[0]).toBe("Great");
    });
});

describe("validateDraft", () => {
    it("accepts a survey inside every limit", () => {
        // when
        const refusal = validateDraft({
            title: " Sprint health ",
            questions: [choice(), scale({ scale_min: 0, scale_max: 10 })],
        });

        // then
        expect(refusal).toBeNull();
    });

    it.each([
        { name: "no title", title: "  ", questions: [choice()], want: "Give the survey a title." },
        {
            name: "a long title",
            title: "t".repeat(121),
            questions: [choice()],
            want: "A survey title can be at most 120 characters.",
        },
        { name: "no questions", title: "Mood", questions: [], want: "Add at least one question." },
        {
            name: "twenty-one questions",
            title: "Mood",
            questions: Array.from({ length: 21 }, () => choice()),
            want: "A survey can have at most 20 questions.",
        },
        {
            name: "a blank prompt",
            title: "Mood",
            questions: [choice({ prompt: " " })],
            want: "Question 1 needs a prompt.",
        },
        {
            name: "a long prompt",
            title: "Mood",
            questions: [scale(), scale({ prompt: "p".repeat(301) })],
            want: "Question 2's prompt can be at most 300 characters.",
        },
        {
            name: "an empty option",
            title: "Mood",
            questions: [choice({ options: ["Good", " "] })],
            want: "Question 1 has an empty option.",
        },
        {
            name: "one option",
            title: "Mood",
            questions: [choice({ options: ["Good"] })],
            want: "Question 1 needs at least 2 options.",
        },
        {
            name: "eleven options",
            title: "Mood",
            questions: [choice({ options: Array.from({ length: 11 }, (_, index) => `Option ${index}`) })],
            want: "Question 1 can have at most 10 options.",
        },
        {
            name: "a long option",
            title: "Mood",
            questions: [choice({ options: ["Good", "o".repeat(101)] })],
            want: "Each option of question 1 can be at most 100 characters.",
        },
        {
            name: "the same option twice",
            title: "Mood",
            questions: [choice({ options: ["Good", " good"] })],
            want: "Question 1 lists the same option twice.",
        },
        {
            name: "a scale upside down",
            title: "Mood",
            questions: [scale({ scale_min: 5, scale_max: 1 })],
            want: "Question 1's scale must run from a lower to a higher whole number between 0 and 10.",
        },
        {
            name: "a scale past ten",
            title: "Mood",
            questions: [scale({ scale_max: 11 })],
            want: "Question 1's scale must run from a lower to a higher whole number between 0 and 10.",
        },
        {
            name: "a scale with a fraction",
            title: "Mood",
            questions: [scale({ scale_min: 1.5 })],
            want: "Question 1's scale must run from a lower to a higher whole number between 0 and 10.",
        },
    ])("refuses $name", ({ title, questions, want }) => {
        // when
        const refusal = validateDraft({ title, questions });

        // then
        expect(refusal).toBe(want);
    });
});

describe("draftRequest", () => {
    it("trims the text and sends each kind only the fields it takes", () => {
        // when
        const request = draftRequest({
            title: " Sprint health ",
            questions: [
                scale({ prompt: " Pace? ", options: ["left over"] }),
                choice({ prompt: "Mood? ", options: [" Good ", "Bad"] }),
            ],
        });

        // then
        expect(request).toEqual({
            title: "Sprint health",
            questions: [
                { kind: "scale", prompt: "Pace?", options: [], scale_min: 1, scale_max: 5 },
                { kind: "single_choice", prompt: "Mood?", options: ["Good", "Bad"], scale_min: null, scale_max: null },
            ],
        });
    });
});

describe("scaleValues", () => {
    it("lists every value from the lowest to the highest", () => {
        // when
        const values = scaleValues({ scale_min: 0, scale_max: 3 });

        // then
        expect(values).toEqual([0, 1, 2, 3]);
    });

    it("lists nothing for a choice question", () => {
        // when
        const values = scaleValues(mood);

        // then
        expect(values).toEqual([]);
    });
});

describe("pick", () => {
    it("replaces the pick of a single choice and a scale", () => {
        // when
        const single = pick(mood, [0], 2);
        const rating = pick(pace, [3], 5);

        // then
        expect(single).toEqual([2]);
        expect(rating).toEqual([5]);
    });

    it("toggles a pick of a multiple choice and keeps them in order", () => {
        // when
        const added = pick(focus, [2], 0);
        const removed = pick(focus, [0, 2], 2);

        // then
        expect(added).toEqual([0, 2]);
        expect(removed).toEqual([0]);
    });
});

describe("answersComplete and surveyAnswers", () => {
    const survey = makeSurvey({ questions: [pace, mood, focus] });

    it("is complete only once every question has a pick", () => {
        // when
        const partial = answersComplete(survey, { "q-pace": [3], "q-focus": [] });
        const whole = answersComplete(survey, { "q-pace": [3], "q-mood": [1], "q-focus": [0] });

        // then
        expect(partial).toBe(false);
        expect(whole).toBe(true);
    });

    it("sends the picked answers in question order", () => {
        // when
        const answers = surveyAnswers(survey, { "q-focus": [0, 2], "q-pace": [4], "q-mood": [] });

        // then
        expect(answers).toEqual([
            { question_id: "q-pace", answer: [4] },
            { question_id: "q-focus", answer: [0, 2] },
        ]);
    });
});

describe("resultRows", () => {
    it("labels a scale's counts by value, with each share of the responses", () => {
        // given
        const closed = makeSurveyQuestion({ results: { counts: [0, 1, 0, 3, 0], responses: 4, average: 3.5 } });

        // when
        const rows = resultRows(closed);

        // then
        expect(rows).toEqual([
            { label: "1", count: 0, share: 0 },
            { label: "2", count: 1, share: 0.25 },
            { label: "3", count: 0, share: 0 },
            { label: "4", count: 3, share: 0.75 },
            { label: "5", count: 0, share: 0 },
        ]);
        expect(rows.map(sharePercent)).toEqual([0, 25, 0, 75, 0]);
    });

    it("labels a choice's counts by option", () => {
        // given
        const closed = { ...focus, results: { counts: [2, 1, 2], responses: 2, average: null } };

        // when
        const rows = resultRows(closed);

        // then
        expect(rows.map(row => [row.label, row.count, row.share])).toEqual([
            ["Quality", 2, 1],
            ["Speed", 1, 0.5],
            ["Process", 2, 1],
        ]);
    });

    it("gives every row no share when nobody answered", () => {
        // given
        const closed = { ...mood, results: { counts: [0, 0, 0], responses: 0, average: null } };

        // when
        const rows = resultRows(closed);

        // then
        expect(rows.every(row => row.share === 0)).toBe(true);
    });

    it("has no rows while the results are hidden", () => {
        // when
        const rows = resultRows(pace);

        // then
        expect(rows).toEqual([]);
    });
});

describe("formatAverage and respondentsLabel", () => {
    it("writes an average with at most two decimals", () => {
        // when
        const shown = [formatAverage(3.5), formatAverage(4), formatAverage(2.67), formatAverage(null)];

        // then
        expect(shown).toEqual(["3.5", "4", "2.67", "No scores yet"]);
    });

    it("counts the people who answered", () => {
        // when
        const shown = [respondentsLabel(0), respondentsLabel(1), respondentsLabel(7)];

        // then
        expect(shown).toEqual(["0 people answered", "1 person answered", "7 people answered"]);
    });
});

describe("surveyRefusal", () => {
    it.each([
        { code: "survey_closed", want: "This survey has closed, so it takes no more answers." },
        { code: "too_many_surveys", want: "A board can hold at most 20 surveys." },
        { code: "invalid_scale", want: "A scale must run from a lower to a higher whole number between 0 and 10." },
        { code: "unknown_survey_template", want: "That survey template is no longer offered." },
    ])("explains $code", ({ code, want }) => {
        // when
        const message = surveyRefusal(makeApiError({ status: 409, message: "refused", code }));

        // then
        expect(message).toBe(want);
    });

    it("falls back to the server's own words", () => {
        // when
        const message = surveyRefusal(makeApiError({ status: 404, message: "survey not found" }));

        // then
        expect(message).toBe("survey not found");
    });
});

describe("templatesByCategory", () => {
    it("groups health checks first and leaves out an empty group", () => {
        // when
        const both = templatesByCategory([teamMood, healthCheck]);
        const surveysOnly = templatesByCategory([teamMood]);

        // then
        expect(both.map(group => [group.label, group.templates.map(template => template.key)])).toEqual([
            ["Health check", ["health_check"]],
            ["Survey", ["team_mood"]],
        ]);
        expect(surveysOnly.map(group => group.label)).toEqual(["Survey"]);
    });
});
