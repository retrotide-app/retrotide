import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeSavedTemplate } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useSavedTemplates } from "./savedTemplates";

const endpoints = vi.hoisted(() => ({
    listSavedTemplates: vi.fn(),
}));

vi.mock("../../api/endpoints/savedTemplates", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listSavedTemplates.mockResolvedValue({ templates: [] });
});

describe("useSavedTemplates", () => {
    it("reads the organisation's saved templates under their key", async () => {
        // given
        const listing = { templates: [makeSavedTemplate(), makeSavedTemplate({ id: "s2", scope: "team" })] };
        endpoints.listSavedTemplates.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useSavedTemplates("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listSavedTemplates).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.savedTemplates("o1"))).toEqual(listing);
    });

    it("asks nothing without an organisation id", () => {
        // when
        const { result } = setup(() => useSavedTemplates(""));

        // then
        expect(endpoints.listSavedTemplates).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});
