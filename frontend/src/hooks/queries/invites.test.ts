import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeInviteDetails, makeOrganisationInvite } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useInviteLookup, useOrganisationInvites } from "./invites";

const endpoints = vi.hoisted(() => ({
    listInvites: vi.fn(),
    lookUpInvite: vi.fn(),
}));

vi.mock("../../api/endpoints/invites", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listInvites.mockResolvedValue({ invites: [] });
    endpoints.lookUpInvite.mockResolvedValue(makeInviteDetails());
});

describe("useOrganisationInvites", () => {
    it("reads the pending invites under the organisation's invites key", async () => {
        // given
        const listing = { invites: [makeOrganisationInvite(), makeOrganisationInvite({ id: "i2", expired: true })] };
        endpoints.listInvites.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useOrganisationInvites("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listInvites).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.invites("o1"))).toEqual(listing);
    });

    it("asks nothing while it is switched off, as for a member who may not manage invites", () => {
        // when
        const { result } = setup(() => useOrganisationInvites("o1", false));

        // then
        expect(endpoints.listInvites).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    it("asks nothing without an organisation id", () => {
        // when
        setup(() => useOrganisationInvites(""));

        // then
        expect(endpoints.listInvites).not.toHaveBeenCalled();
    });
});

describe("useInviteLookup", () => {
    it("looks the invite up by its token under the lookup key", async () => {
        // given
        const details = makeInviteDetails({ team_name: "Crew", email: "kit@example.test" });
        endpoints.lookUpInvite.mockResolvedValue(details);

        // when
        const { result, queryClient } = setup(() => useInviteLookup("invite-1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(details));
        expect(endpoints.lookUpInvite).toHaveBeenCalledWith({ token: "invite-1" });
        expect(queryClient.getQueryData(queryKeys.invites.lookup("invite-1"))).toEqual(details);
    });

    it("hands back the refusal of a used or expired invite as it comes", async () => {
        // given
        const failure = makeApiError({ status: 400, message: "the invite is not valid", code: "invite_invalid" });
        endpoints.lookUpInvite.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useInviteLookup("invite-1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
        expect(result.current.data).toBeUndefined();
    });

    it("asks nothing without a token", () => {
        // when
        const { result } = setup(() => useInviteLookup(""));

        // then
        expect(endpoints.lookUpInvite).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});
