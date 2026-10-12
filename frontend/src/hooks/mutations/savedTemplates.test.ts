import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeSavedTemplate } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useDeleteSavedTemplate, useSaveBoardAsTemplate } from "./savedTemplates";

const endpoints = vi.hoisted(() => ({
    deleteSavedTemplate: vi.fn(),
    listSavedTemplates: vi.fn(),
    saveBoardAsTemplate: vi.fn(),
}));

vi.mock("../../api/endpoints/savedTemplates", () => endpoints);

const HARBOUR_TEMPLATES = queryKeys.organisations.savedTemplates("o1");

const HARBOUR_ENTITLEMENTS = queryKeys.organisations.entitlements("o1");

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient, invalidate };
}

beforeEach(() => {
    endpoints.saveBoardAsTemplate.mockResolvedValue(makeSavedTemplate());
    endpoints.deleteSavedTemplate.mockResolvedValue(undefined);
});

describe("useSaveBoardAsTemplate", () => {
    it("saves the board's columns and refreshes the templates and the plan usage", async () => {
        // given
        const { result, queryClient, invalidate } = setup(useSaveBoardAsTemplate);

        // when
        await act(async () => {
            await result.current.mutateAsync({
                orgId: "o1",
                boardId: "b1",
                name: "Our retro",
                description: "",
                scope: "personal",
            });
        });

        // then
        expect(endpoints.saveBoardAsTemplate).toHaveBeenCalledWith("b1", {
            name: "Our retro",
            description: "",
            scope: "personal",
        });
        expectInvalidated(queryClient, HARBOUR_TEMPLATES);
        expectInvalidated(queryClient, HARBOUR_ENTITLEMENTS);
        expect(invalidate).toHaveBeenCalledTimes(2);
    });

    it("refreshes after an upgrade prompt, because the shown usage is out of date", async () => {
        // given
        endpoints.saveBoardAsTemplate.mockRejectedValue(makeApiError({ status: 402, message: "upgrade required" }));
        const { result, queryClient } = setup(useSaveBoardAsTemplate);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({
                    orgId: "o1",
                    boardId: "b1",
                    name: "Our retro",
                    description: "",
                    scope: "personal",
                }),
            ).rejects.toThrow("upgrade required");
        });

        // then
        expectInvalidated(queryClient, HARBOUR_ENTITLEMENTS);
    });

    it("leaves every cache alone when the server refuses", async () => {
        // given
        endpoints.saveBoardAsTemplate.mockRejectedValue(
            makeApiError({ status: 403, message: "you cannot save this board as a template" }),
        );
        const { result, invalidate } = setup(useSaveBoardAsTemplate);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({
                    orgId: "o1",
                    boardId: "b1",
                    name: "Our retro",
                    description: "",
                    scope: "team",
                }),
            ).rejects.toThrow("you cannot save this board as a template");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useDeleteSavedTemplate", () => {
    it("deletes the template and refreshes the templates and the plan usage", async () => {
        // given
        const { result, queryClient, invalidate } = setup(() => useDeleteSavedTemplate("o1"));

        // when
        await act(async () => {
            await result.current.mutateAsync("s1");
        });

        // then
        expect(endpoints.deleteSavedTemplate).toHaveBeenCalledWith("o1", "s1");
        expectInvalidated(queryClient, HARBOUR_TEMPLATES);
        expectInvalidated(queryClient, HARBOUR_ENTITLEMENTS);
        expect(invalidate).toHaveBeenCalledTimes(2);
    });

    it("refreshes when the template is already gone", async () => {
        // given
        endpoints.deleteSavedTemplate.mockRejectedValue(makeApiError({ status: 404, message: "template not found" }));
        const { result, queryClient } = setup(() => useDeleteSavedTemplate("o1"));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync("s1")).rejects.toThrow("template not found");
        });

        // then
        expectInvalidated(queryClient, HARBOUR_TEMPLATES);
    });

    it("leaves every cache alone when deleting is not allowed", async () => {
        // given
        endpoints.deleteSavedTemplate.mockRejectedValue(
            makeApiError({ status: 403, message: "you cannot delete this template" }),
        );
        const { result, invalidate } = setup(() => useDeleteSavedTemplate("o1"));

        // when
        await act(async () => {
            await expect(result.current.mutateAsync("s1")).rejects.toThrow("you cannot delete this template");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});
