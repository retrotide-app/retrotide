import { beforeEach, describe, vi } from "vitest";
import * as api from "./surveys";
import {
    deleteWithBodyMock,
    fetchMock,
    postMock,
    putMock,
    resetTransports,
    runRequestCases,
    type RequestCase,
} from "./testHarness";

vi.mock("../client", async importOriginal => {
    const actual = await importOriginal<typeof import("../client")>();

    return {
        ...actual,
        apiFetch: vi.fn(),
        apiPost: vi.fn(),
        apiPut: vi.fn(),
        apiPatch: vi.fn(),
        apiDelete: vi.fn(),
        apiDeleteWithBody: vi.fn(),
    };
});

beforeEach(resetTransports);

describe("the survey templates", () => {
    const cases: RequestCase[] = [
        {
            name: "listSurveyTemplates reads the public catalogue",
            call: () => api.listSurveyTemplates(),
            transport: fetchMock,
            request: ["/survey-templates"],
            strict: true,
        },
    ];

    runRequestCases(cases);
});

describe("a board's surveys", () => {
    const question = { id: "q1", kind: "scale" as const, prompt: "Pace?", scale_min: 1, scale_max: 5 };

    const cases: RequestCase[] = [
        {
            name: "createSurvey posts a survey from a template",
            call: () =>
                api.createSurvey("b/1", { op_id: "op1", id: "s1", title: "Team mood", template_key: "team_mood" }),
            transport: postMock,
            request: [
                "/boards/b%2F1/surveys",
                { op_id: "op1", id: "s1", title: "Team mood", template_key: "team_mood" },
            ],
            strict: true,
        },
        {
            name: "createSurvey posts a survey with its own questions",
            call: () => api.createSurvey("b1", { op_id: "op1", id: "s1", title: "Pace", questions: [question] }),
            transport: postMock,
            request: ["/boards/b1/surveys", { op_id: "op1", id: "s1", title: "Pace", questions: [question] }],
            strict: true,
        },
        {
            name: "closeSurvey posts to the survey's close action",
            call: () => api.closeSurvey("b1", "s/1", { op_id: "op1" }),
            transport: postMock,
            request: ["/boards/b1/surveys/s%2F1/close", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "deleteSurvey deletes the survey with its op id",
            call: () => api.deleteSurvey("b1", "s1", { op_id: "op1" }),
            transport: deleteWithBodyMock,
            request: ["/boards/b1/surveys/s1", { op_id: "op1" }],
            strict: true,
        },
        {
            name: "respondToSurvey puts every answer",
            call: () =>
                api.respondToSurvey("b1", "s1", { op_id: "op1", answers: [{ question_id: "q1", answer: [4] }] }),
            transport: putMock,
            request: [
                "/boards/b1/surveys/s1/responses",
                { op_id: "op1", answers: [{ question_id: "q1", answer: [4] }] },
            ],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
