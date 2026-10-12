import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeBoardCommandResponse, makeBoardSnapshot } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useCloseSurvey, useCreateSurvey, useDeleteSurvey, useRespondToSurvey } from "./surveys";

const endpoints = vi.hoisted(() => ({
    listSurveyTemplates: vi.fn(),
    createSurvey: vi.fn(),
    closeSurvey: vi.fn(),
    deleteSurvey: vi.fn(),
    respondToSurvey: vi.fn(),
}));

vi.mock("../../api/endpoints/surveys", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const anyId = expect.stringMatching(V4);

const BOARD = "b1";

const BOARD_STATE = queryKeys.boardLive.state(BOARD);

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
}

const commands: CommandCase[] = [
    {
        name: "useCreateSurvey creates a survey from a template under a new v4 id",
        use: useCreateSurvey,
        variables: { title: "Team mood", template_key: "team_mood" },
        endpoint: endpoints.createSurvey,
        request: [BOARD, { op_id: anyId, id: anyId, title: "Team mood", template_key: "team_mood" }],
    },
    {
        name: "useCreateSurvey gives every custom question its own v4 id",
        use: useCreateSurvey,
        variables: {
            title: "Pace",
            questions: [{ kind: "scale", prompt: "Pace?", options: [], scale_min: 1, scale_max: 5 }],
        },
        endpoint: endpoints.createSurvey,
        request: [
            BOARD,
            {
                op_id: anyId,
                id: anyId,
                title: "Pace",
                questions: [{ id: anyId, kind: "scale", prompt: "Pace?", options: [], scale_min: 1, scale_max: 5 }],
            },
        ],
    },
    {
        name: "useCloseSurvey closes the survey",
        use: useCloseSurvey,
        variables: { surveyId: "s1" },
        endpoint: endpoints.closeSurvey,
        request: [BOARD, "s1", { op_id: anyId }],
    },
    {
        name: "useDeleteSurvey deletes the survey",
        use: useDeleteSurvey,
        variables: { surveyId: "s1" },
        endpoint: endpoints.deleteSurvey,
        request: [BOARD, "s1", { op_id: anyId }],
    },
    {
        name: "useRespondToSurvey sends every answer",
        use: useRespondToSurvey,
        variables: { surveyId: "s1", answers: [{ question_id: "q1", answer: [4] }] },
        endpoint: endpoints.respondToSurvey,
        request: [BOARD, "s1", { op_id: anyId, answers: [{ question_id: "q1", answer: [4] }] }],
    },
];

function renderCommand<T>(use: (boardId: string) => T) {
    const qc = createTestQueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    const rendered = renderHook(() => use(BOARD), { wrapper: providerWrapper({ queryClient: qc }) });

    return { ...rendered, qc, invalidate };
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
        fn.mockResolvedValue(makeBoardCommandResponse());
    }
});

describe("survey commands", () => {
    it.each(commands)(
        "$name, and leaves the board for its own frame",
        async ({ use, variables, endpoint, request }) => {
            // given
            const { result, qc, invalidate } = renderCommand(use);
            const held = makeBoardSnapshot({ board_id: BOARD, version: 3 });
            qc.setQueryDefaults(BOARD_STATE, { gcTime: Infinity });
            qc.setQueryData(BOARD_STATE, held);

            // when
            await act(async () => {
                await result.current.mutateAsync(variables as never);
            });

            // then
            expect(endpoint.mock.calls[0]).toEqual(request);
            expect(qc.getQueryData(BOARD_STATE)).toBe(held);
            expect(invalidate).not.toHaveBeenCalled();
        },
    );

    it.each(commands)("files the command under the board's mutation key: $name", async ({ use, variables }) => {
        // given
        const { result, qc } = renderCommand(use);

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
        });

        // then
        const filed = qc.getMutationCache().findAll({ mutationKey: queryKeys.boardLive.mutations(BOARD) });
        expect(filed).toHaveLength(1);
    });

    it.each([403, 404, 409])("refetches the board after a %i", async status => {
        // given
        endpoints.respondToSurvey.mockRejectedValue(makeApiError({ status, message: "the survey is closed" }));
        const { result, qc, invalidate } = renderCommand(useRespondToSurvey);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({ surveyId: "s1", answers: [{ question_id: "q1", answer: [1] }] }),
            ).rejects.toThrow("the survey is closed");
        });

        // then
        expectInvalidated(qc, BOARD_STATE);
        expect(invalidate).toHaveBeenCalledTimes(1);
    });

    it("leaves everything alone when a question is refused", async () => {
        // given
        endpoints.createSurvey.mockRejectedValue(
            makeApiError({ status: 400, message: "every question needs a prompt", code: "question_prompt_required" }),
        );
        const { result, invalidate } = renderCommand(useCreateSurvey);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({
                    title: "Pace",
                    questions: [{ kind: "scale", prompt: "", scale_min: 1, scale_max: 5 }],
                }),
            ).rejects.toThrow("every question needs a prompt");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it("gives every new survey its own id", async () => {
        // given
        const { result } = renderCommand(useCreateSurvey);

        // when
        await act(async () => {
            await result.current.mutateAsync({ title: "Mood", template_key: "team_mood" });
            await result.current.mutateAsync({ title: "Mood", template_key: "team_mood" });
        });

        // then
        const ids = endpoints.createSurvey.mock.calls.map(call => (call[1] as { id: string }).id);
        expect(ids[0]).not.toBe(ids[1]);
    });
});
