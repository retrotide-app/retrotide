import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import type { SurveyTemplateListResponse } from "../../types/api";
import { useSurveyTemplates } from "./surveys";

const endpoints = vi.hoisted(() => ({
    listSurveyTemplates: vi.fn(),
}));

vi.mock("../../api/endpoints/surveys", () => endpoints);

const catalogue: SurveyTemplateListResponse = {
    templates: [
        {
            key: "team_mood",
            title: "Team mood",
            description: "A single pick of how everyone is feeling.",
            category: "survey",
            questions: [
                {
                    kind: "single_choice",
                    prompt: "How are you feeling?",
                    options: ["Good", "Bad"],
                    scale_min: null,
                    scale_max: null,
                },
            ],
        },
    ],
};

function setup(enabled?: boolean) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(() => useSurveyTemplates(enabled), { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listSurveyTemplates.mockReset();
    endpoints.listSurveyTemplates.mockResolvedValue(catalogue);
});

describe("useSurveyTemplates", () => {
    it("reads the template catalogue under its key", async () => {
        // when
        const { result, queryClient } = setup();

        // then
        await waitFor(() => expect(result.current.data).toEqual(catalogue));
        expect(queryClient.getQueryData(queryKeys.surveyTemplates())).toEqual(catalogue);
    });

    it("keeps the catalogue fresh, so a second reader asks nothing more", async () => {
        // given
        const { result, queryClient } = setup();
        await waitFor(() => expect(result.current.data).toEqual(catalogue));

        // when
        const again = renderHook(() => useSurveyTemplates(), { wrapper: providerWrapper({ queryClient }) });

        // then
        expect(again.result.current.data).toEqual(catalogue);
        expect(endpoints.listSurveyTemplates).toHaveBeenCalledTimes(1);
    });

    it("asks nothing while it is switched off", () => {
        // when
        const { result } = setup(false);

        // then
        expect(result.current.data).toBeUndefined();
        expect(endpoints.listSurveyTemplates).not.toHaveBeenCalled();
    });

    it("hands back the error when the catalogue could not be read", async () => {
        // given
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        endpoints.listSurveyTemplates.mockRejectedValue(failure);

        // when
        const { result } = setup();

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});
