import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import {
    makeAdminOrganisation,
    makeAdminOrganisationDetail,
    makeAdminOrganisationList,
    makeApiError,
} from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useAdminOrganisation, useAdminOrganisations } from "./adminOrganisations";

const endpoints = vi.hoisted(() => ({
    getAdminOrganisations: vi.fn(),
    getAdminOrganisation: vi.fn(),
}));

vi.mock("../../api/endpoints/adminOrganisations", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getAdminOrganisations.mockResolvedValue(makeAdminOrganisationList([]));
    endpoints.getAdminOrganisation.mockResolvedValue(makeAdminOrganisationDetail());
});

describe("useAdminOrganisations", () => {
    it("reads a page of organisations under the key for its search and page", async () => {
        // given
        const query = { search: "harbour", limit: 20, offset: 0 };
        const listing = makeAdminOrganisationList([makeAdminOrganisation()]);
        endpoints.getAdminOrganisations.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useAdminOrganisations(query));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.getAdminOrganisations).toHaveBeenCalledWith(query);
        expect(queryClient.getQueryData(queryKeys.admin.organisations.list(query))).toEqual(listing);
    });

    it("keeps the last page on show while the next one loads", async () => {
        // given
        const first = makeAdminOrganisationList([makeAdminOrganisation({ name: "Harbour" })]);
        endpoints.getAdminOrganisations.mockResolvedValueOnce(first).mockReturnValueOnce(new Promise(() => {}));
        const { result, rerender } = renderHook(({ offset }) => useAdminOrganisations({ offset }), {
            wrapper: providerWrapper(),
            initialProps: { offset: 0 },
        });
        await waitFor(() => expect(result.current.data).toEqual(first));

        // when
        rerender({ offset: 20 });

        // then
        expect(result.current.data).toEqual(first);
        expect(result.current.loading).toBe(false);
    });

    it("hands back the error when the organisations could not be listed", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to list the organisations" });
        endpoints.getAdminOrganisations.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useAdminOrganisations({}));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});

describe("useAdminOrganisation", () => {
    it("reads one organisation under that organisation's key", async () => {
        // given
        const detail = makeAdminOrganisationDetail({ id: "o1" });
        endpoints.getAdminOrganisation.mockResolvedValue(detail);

        // when
        const { result, queryClient } = setup(() => useAdminOrganisation("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(detail));
        expect(endpoints.getAdminOrganisation).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.admin.organisations.detail("o1"))).toEqual(detail);
    });

    it("asks nothing without an organisation id", () => {
        // when
        const { result } = setup(() => useAdminOrganisation(""));

        // then
        expect(endpoints.getAdminOrganisation).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    it("hands back the error when the organisation could not load", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "organisation not found" });
        endpoints.getAdminOrganisation.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useAdminOrganisation("o1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});
