import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeEntitlements, makeOrganisation, makeOrganisationMember } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useEntitlements, useOrganisation, useOrganisationMembers, useOrganisations } from "./organisations";

const endpoints = vi.hoisted(() => ({
    getEntitlements: vi.fn(),
    getOrganisation: vi.fn(),
    listOrganisationMembers: vi.fn(),
    listOrganisations: vi.fn(),
}));

vi.mock("../../api/endpoints/organisations", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listOrganisations.mockResolvedValue({ organisations: [] });
    endpoints.getOrganisation.mockResolvedValue(makeOrganisation());
    endpoints.listOrganisationMembers.mockResolvedValue({ members: [] });
    endpoints.getEntitlements.mockResolvedValue(makeEntitlements());
});

describe("useOrganisations", () => {
    it("reads the caller's organisations under the list key", async () => {
        // given
        const listing = { organisations: [makeOrganisation()] };
        endpoints.listOrganisations.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useOrganisations());

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(queryClient.getQueryData(queryKeys.organisations.list())).toEqual(listing);
    });

    it("asks nothing while it is switched off, as for a signed out visitor", () => {
        // when
        const { result } = setup(() => useOrganisations(false));

        // then
        expect(endpoints.listOrganisations).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    it("hands back the error when the organisations could not be listed", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to list your organisations" });
        endpoints.listOrganisations.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useOrganisations());

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });
});

describe("useOrganisation", () => {
    it("reads one organisation under its detail key", async () => {
        // given
        const organisation = makeOrganisation({ id: "o1" });
        endpoints.getOrganisation.mockResolvedValue(organisation);

        // when
        const { result, queryClient } = setup(() => useOrganisation("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(organisation));
        expect(endpoints.getOrganisation).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.detail("o1"))).toEqual(organisation);
    });

    it("asks nothing without an organisation id", () => {
        // when
        const { result } = setup(() => useOrganisation(""));

        // then
        expect(endpoints.getOrganisation).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    it("hands back a not found answer as it comes, as for a deleted organisation", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "organisation not found" });
        endpoints.getOrganisation.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useOrganisation("o1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});

describe("useOrganisationMembers", () => {
    it("reads the members under the organisation's members key", async () => {
        // given
        const listing = { members: [makeOrganisationMember()] };
        endpoints.listOrganisationMembers.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useOrganisationMembers("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listOrganisationMembers).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.members("o1"))).toEqual(listing);
    });

    it("asks nothing without an organisation id", () => {
        // when
        setup(() => useOrganisationMembers(""));

        // then
        expect(endpoints.listOrganisationMembers).not.toHaveBeenCalled();
    });
});

describe("useEntitlements", () => {
    it("reads the plan and its limits under the organisation's entitlements key", async () => {
        // given
        const entitlements = makeEntitlements();
        endpoints.getEntitlements.mockResolvedValue(entitlements);

        // when
        const { result, queryClient } = setup(() => useEntitlements("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(entitlements));
        expect(endpoints.getEntitlements).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.entitlements("o1"))).toEqual(entitlements);
    });

    it("asks nothing without an organisation id", () => {
        // when
        setup(() => useEntitlements(""));

        // then
        expect(endpoints.getEntitlements).not.toHaveBeenCalled();
    });
});
