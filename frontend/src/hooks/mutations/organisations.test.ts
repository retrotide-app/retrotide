import type { QueryKey } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeOrganisation, makeWorkspace } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useChangeMemberRole,
    useCreateWorkspace,
    useDeleteOrganisation,
    useLeaveOrganisation,
    useRemoveMember,
    useRestoreOrganisation,
    useTransferOwnership,
    useUpdateOrganisation,
} from "./organisations";

const endpoints = vi.hoisted(() => ({
    changeMemberRole: vi.fn(),
    createWorkspace: vi.fn(),
    deleteOrganisation: vi.fn(),
    leaveOrganisation: vi.fn(),
    removeMember: vi.fn(),
    restoreOrganisation: vi.fn(),
    transferOwnership: vi.fn(),
    updateOrganisation: vi.fn(),
}));

vi.mock("../../api/endpoints/organisations", () => endpoints);

interface MutationCase {
    name: string;
    use: () => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
    invalidated: QueryKey[];
}

interface OutdatedCase {
    name: string;
    use: () => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    status: number;
    invalidated: QueryKey[];
}

const LIST = queryKeys.organisations.list();

const HARBOUR = queryKeys.organisations.scope("o1");

const cases: MutationCase[] = [
    {
        name: "useCreateWorkspace creates the organisation and its first team and refreshes the list",
        use: useCreateWorkspace,
        variables: { name: "Harbour", team_name: "Crew" },
        endpoint: endpoints.createWorkspace,
        request: [{ name: "Harbour", team_name: "Crew" }],
        invalidated: [LIST],
    },
    {
        name: "useUpdateOrganisation saves the name, address and guest policy and refreshes the list and the organisation",
        use: useUpdateOrganisation,
        variables: { orgId: "o1", name: "Harbour", slug: "harbour", allow_guests: false },
        endpoint: endpoints.updateOrganisation,
        request: ["o1", { name: "Harbour", slug: "harbour", allow_guests: false }],
        invalidated: [LIST, queryKeys.organisations.detail("o1")],
    },
    {
        name: "useDeleteOrganisation deletes the organisation and refreshes the list and everything under it",
        use: useDeleteOrganisation,
        variables: { orgId: "o1" },
        endpoint: endpoints.deleteOrganisation,
        request: ["o1"],
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useRestoreOrganisation restores the organisation and refreshes the list and everything under it",
        use: useRestoreOrganisation,
        variables: { orgId: "o1" },
        endpoint: endpoints.restoreOrganisation,
        request: ["o1"],
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useTransferOwnership hands the organisation over and refreshes the roles it changed",
        use: useTransferOwnership,
        variables: { orgId: "o1", user_id: "u2" },
        endpoint: endpoints.transferOwnership,
        request: ["o1", { user_id: "u2" }],
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useLeaveOrganisation leaves the organisation and refreshes the list",
        use: useLeaveOrganisation,
        variables: { orgId: "o1" },
        endpoint: endpoints.leaveOrganisation,
        request: ["o1"],
        invalidated: [LIST],
    },
    {
        name: "useChangeMemberRole changes the member's role and refreshes the members",
        use: useChangeMemberRole,
        variables: { orgId: "o1", userId: "u2", role: "admin" },
        endpoint: endpoints.changeMemberRole,
        request: ["o1", "u2", { role: "admin" }],
        invalidated: [queryKeys.organisations.members("o1")],
    },
    {
        name: "useRemoveMember removes the member and refreshes the members and every team, which lost them too",
        use: useRemoveMember,
        variables: { orgId: "o1", userId: "u2" },
        endpoint: endpoints.removeMember,
        request: ["o1", "u2"],
        invalidated: [queryKeys.organisations.members("o1"), queryKeys.teams.all],
    },
];

const outdated: OutdatedCase[] = [
    {
        name: "useDeleteOrganisation, whose organisation is already gone",
        use: useDeleteOrganisation,
        variables: { orgId: "o1" },
        endpoint: endpoints.deleteOrganisation,
        status: 404,
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useRestoreOrganisation, whose organisation was restored meanwhile or ran out of time",
        use: useRestoreOrganisation,
        variables: { orgId: "o1" },
        endpoint: endpoints.restoreOrganisation,
        status: 409,
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useTransferOwnership, when the new owner has left",
        use: useTransferOwnership,
        variables: { orgId: "o1", user_id: "u2" },
        endpoint: endpoints.transferOwnership,
        status: 404,
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useTransferOwnership, when the organisation or its owner changed meanwhile",
        use: useTransferOwnership,
        variables: { orgId: "o1", user_id: "u2" },
        endpoint: endpoints.transferOwnership,
        status: 409,
        invalidated: [LIST, HARBOUR],
    },
    {
        name: "useLeaveOrganisation, whose organisation is gone",
        use: useLeaveOrganisation,
        variables: { orgId: "o1" },
        endpoint: endpoints.leaveOrganisation,
        status: 404,
        invalidated: [LIST],
    },
    {
        name: "useChangeMemberRole, whose member has left",
        use: useChangeMemberRole,
        variables: { orgId: "o1", userId: "u2", role: "member" },
        endpoint: endpoints.changeMemberRole,
        status: 404,
        invalidated: [queryKeys.organisations.members("o1")],
    },
    {
        name: "useRemoveMember, whose member has already left",
        use: useRemoveMember,
        variables: { orgId: "o1", userId: "u2" },
        endpoint: endpoints.removeMember,
        status: 404,
        invalidated: [queryKeys.organisations.members("o1"), queryKeys.teams.all],
    },
];

const STALE_CACHE_MESSAGE =
    "the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply";

const HIERARCHY_REFUSAL = "nobody may act on someone at their own level or above";

function keepingClient() {
    const qc = createTestQueryClient();
    const defaults = qc.getDefaultOptions();
    qc.setDefaultOptions({ ...defaults, queries: { ...defaults.queries, gcTime: Infinity } });

    return qc;
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockResolvedValue(undefined);
    }

    endpoints.createWorkspace.mockResolvedValue(makeWorkspace());
});

describe("organisation mutations", () => {
    it.each(cases)("$name", async ({ use, variables, endpoint, request, invalidated }) => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
        });

        // then
        expect(endpoint.mock.calls[0].slice(0, request.length)).toEqual(request);
        for (const key of invalidated) {
            expectInvalidated(qc, key);
        }
        expect(invalidate).toHaveBeenCalledTimes(invalidated.length);
    });

    it.each(cases)("leaves every cache alone when the server refuses: $name", async ({ use, variables, endpoint }) => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const setData = vi.spyOn(qc, "setQueryData");
        const remove = vi.spyOn(qc, "removeQueries");
        endpoint.mockRejectedValue(makeApiError({ status: 403, message: HIERARCHY_REFUSAL }));
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(variables as never)).rejects.toThrow(HIERARCHY_REFUSAL);
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
        expect(setData).not.toHaveBeenCalled();
        expect(remove).not.toHaveBeenCalled();
    });

    it.each(cases)(
        "refreshes after a 500, which the server also answers for a saved change: $name",
        async ({ use, variables, endpoint, invalidated }) => {
            // given
            const qc = createTestQueryClient();
            const invalidate = vi.spyOn(qc, "invalidateQueries");
            endpoint.mockRejectedValue(makeApiError({ status: 500, message: STALE_CACHE_MESSAGE }));
            const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

            // when
            await act(async () => {
                await expect(result.current.mutateAsync(variables as never)).rejects.toThrow(STALE_CACHE_MESSAGE);
            });

            // then
            for (const key of invalidated) {
                expectInvalidated(qc, key);
            }
            expect(invalidate).toHaveBeenCalledTimes(invalidated.length);
        },
    );

    it.each(outdated)(
        "refreshes after a $status, because what is on show is out of date: $name",
        async ({ use, variables, endpoint, status, invalidated }) => {
            // given
            const qc = createTestQueryClient();
            const invalidate = vi.spyOn(qc, "invalidateQueries");
            endpoint.mockRejectedValue(makeApiError({ status, message: "changed meanwhile" }));
            const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

            // when
            await act(async () => {
                await expect(result.current.mutateAsync(variables as never)).rejects.toThrow("changed meanwhile");
            });

            // then
            for (const key of invalidated) {
                expectInvalidated(qc, key);
            }
            expect(invalidate).toHaveBeenCalledTimes(invalidated.length);
        },
    );
});

describe("useCreateWorkspace", () => {
    it("puts the new organisation in the list at once, so nothing sends the user back to onboarding", async () => {
        // given
        const qc = keepingClient();
        const existing = makeOrganisation({ id: "o0", slug: "old", name: "Old" });
        qc.setQueryData(LIST, { organisations: [existing] });
        const workspace = makeWorkspace();
        endpoints.createWorkspace.mockResolvedValue(workspace);
        vi.spyOn(qc, "invalidateQueries").mockResolvedValue(undefined);
        const { result } = renderHook(useCreateWorkspace, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let created: unknown;
        await act(async () => {
            created = await result.current.mutateAsync({ name: "Harbour", team_name: "Crew" });
        });

        // then
        expect(created).toEqual(workspace);
        expect(qc.getQueryData(LIST)).toEqual({ organisations: [existing, workspace.organisation] });
    });

    it("refuses the second workspace of an owner at the allowance and leaves the list alone", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        endpoints.createWorkspace.mockRejectedValue(
            makeApiError({ status: 409, message: "you already own as many organisations as you can" }),
        );
        const { result } = renderHook(useCreateWorkspace, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ name: "Harbour", team_name: "Crew" })).rejects.toThrow(
                "you already own as many organisations as you can",
            );
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
        expect(qc.getQueryData(LIST)).toBeUndefined();
    });
});

describe("useLeaveOrganisation", () => {
    it("drops everything read under the organisation it left instead of fetching it again", async () => {
        // given
        const qc = createTestQueryClient();
        const remove = vi.spyOn(qc, "removeQueries");
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const { result } = renderHook(useLeaveOrganisation, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync({ orgId: "o1" });
        });

        // then
        expect(remove).toHaveBeenCalledWith({ queryKey: HARBOUR });
        expect(remove.mock.invocationCallOrder[0]).toBeLessThan(invalidate.mock.invocationCallOrder[0]);
    });
});
