import type { QueryKey } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeTeam, makeUpgradeRequiredError } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useAddTeamMember,
    useArchiveTeam,
    useChangeTeamMemberRole,
    useCreateTeam,
    useDeleteTeam,
    useRemoveTeamMember,
    useRenameTeam,
} from "./teams";

const endpoints = vi.hoisted(() => ({
    addTeamMember: vi.fn(),
    archiveTeam: vi.fn(),
    changeTeamMemberRole: vi.fn(),
    createTeam: vi.fn(),
    deleteTeam: vi.fn(),
    removeTeamMember: vi.fn(),
    renameTeam: vi.fn(),
}));

vi.mock("../../api/endpoints/teams", () => endpoints);

interface MutationCase {
    name: string;
    use: () => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
    invalidated: QueryKey[];
}

interface OutdatedCase extends Omit<MutationCase, "request"> {
    status: number;
}

const HARBOUR_TEAMS = queryKeys.organisations.teams("o1");

const CREW_MEMBERS = queryKeys.teams.members("t1");

const HARBOUR_BOARDS = queryKeys.boards.organisation("o1");

const EVERY_ORGANISATION_BOARDS = queryKeys.boards.organisations;

const cases: MutationCase[] = [
    {
        name: "useCreateTeam creates the team and refreshes the organisation's teams",
        use: useCreateTeam,
        variables: { orgId: "o1", name: "Deck" },
        endpoint: endpoints.createTeam,
        request: ["o1", { name: "Deck" }],
        invalidated: [HARBOUR_TEAMS],
    },
    {
        name: "useArchiveTeam archives the team and refreshes the organisation's teams, the team and the organisation's boards",
        use: useArchiveTeam,
        variables: { orgId: "o1", teamId: "t1" },
        endpoint: endpoints.archiveTeam,
        request: ["o1", "t1"],
        invalidated: [HARBOUR_TEAMS, queryKeys.teams.scope("t1"), HARBOUR_BOARDS],
    },
    {
        name: "useDeleteTeam deletes the team and refreshes the organisation's teams and boards",
        use: useDeleteTeam,
        variables: { orgId: "o1", teamId: "t1" },
        endpoint: endpoints.deleteTeam,
        request: ["o1", "t1"],
        invalidated: [HARBOUR_TEAMS, HARBOUR_BOARDS],
    },
    {
        name: "useRenameTeam renames the team and refreshes the organisation's teams, the team and the organisation's boards",
        use: useRenameTeam,
        variables: { orgId: "o1", teamId: "t1", name: "Deck" },
        endpoint: endpoints.renameTeam,
        request: ["t1", { name: "Deck" }],
        invalidated: [HARBOUR_TEAMS, queryKeys.teams.detail("t1"), HARBOUR_BOARDS],
    },
    {
        name: "useAddTeamMember adds the member with their team role and refreshes the team's members and the boards",
        use: useAddTeamMember,
        variables: { teamId: "t1", user_id: "u2", role: "member" },
        endpoint: endpoints.addTeamMember,
        request: ["t1", { user_id: "u2", role: "member" }],
        invalidated: [CREW_MEMBERS, EVERY_ORGANISATION_BOARDS],
    },
    {
        name: "useChangeTeamMemberRole changes the team role and refreshes the team's members",
        use: useChangeTeamMemberRole,
        variables: { teamId: "t1", userId: "u2", role: "lead" },
        endpoint: endpoints.changeTeamMemberRole,
        request: ["t1", "u2", { role: "lead" }],
        invalidated: [CREW_MEMBERS],
    },
    {
        name: "useRemoveTeamMember takes the member off the team and refreshes the team's members and the boards",
        use: useRemoveTeamMember,
        variables: { teamId: "t1", userId: "u2" },
        endpoint: endpoints.removeTeamMember,
        request: ["t1", "u2"],
        invalidated: [CREW_MEMBERS, EVERY_ORGANISATION_BOARDS],
    },
];

const outdated: OutdatedCase[] = [
    {
        name: "useArchiveTeam, whose team is already archived",
        use: useArchiveTeam,
        variables: { orgId: "o1", teamId: "t1" },
        endpoint: endpoints.archiveTeam,
        status: 409,
        invalidated: [HARBOUR_TEAMS, queryKeys.teams.scope("t1"), HARBOUR_BOARDS],
    },
    {
        name: "useArchiveTeam, whose team is gone",
        use: useArchiveTeam,
        variables: { orgId: "o1", teamId: "t1" },
        endpoint: endpoints.archiveTeam,
        status: 404,
        invalidated: [HARBOUR_TEAMS, queryKeys.teams.scope("t1"), HARBOUR_BOARDS],
    },
    {
        name: "useDeleteTeam, whose team is already gone",
        use: useDeleteTeam,
        variables: { orgId: "o1", teamId: "t1" },
        endpoint: endpoints.deleteTeam,
        status: 404,
        invalidated: [HARBOUR_TEAMS, HARBOUR_BOARDS],
    },
    {
        name: "useRenameTeam, whose team was archived meanwhile",
        use: useRenameTeam,
        variables: { orgId: "o1", teamId: "t1", name: "Deck" },
        endpoint: endpoints.renameTeam,
        status: 409,
        invalidated: [HARBOUR_TEAMS, queryKeys.teams.detail("t1"), HARBOUR_BOARDS],
    },
    {
        name: "useAddTeamMember, whose member is already on the team",
        use: useAddTeamMember,
        variables: { teamId: "t1", user_id: "u2", role: "member" },
        endpoint: endpoints.addTeamMember,
        status: 409,
        invalidated: [CREW_MEMBERS, EVERY_ORGANISATION_BOARDS],
    },
    {
        name: "useChangeTeamMemberRole, whose member has left the team",
        use: useChangeTeamMemberRole,
        variables: { teamId: "t1", userId: "u2", role: "lead" },
        endpoint: endpoints.changeTeamMemberRole,
        status: 404,
        invalidated: [CREW_MEMBERS],
    },
    {
        name: "useRemoveTeamMember, whose member has already left the team",
        use: useRemoveTeamMember,
        variables: { teamId: "t1", userId: "u2" },
        endpoint: endpoints.removeTeamMember,
        status: 404,
        invalidated: [CREW_MEMBERS, EVERY_ORGANISATION_BOARDS],
    },
];

const STALE_CACHE_MESSAGE =
    "the change was saved, but its cached copy could not be cleared, so it can take up to a minute to apply";

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockResolvedValue(undefined);
    }
});

describe("team mutations", () => {
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
        const remove = vi.spyOn(qc, "removeQueries");
        endpoint.mockRejectedValue(makeApiError({ status: 403, message: "insufficient permissions" }));
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(variables as never)).rejects.toThrow("insufficient permissions");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
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

describe("useCreateTeam", () => {
    it("hands back the new team", async () => {
        // given
        const qc = createTestQueryClient();
        const team = makeTeam({ id: "t2", name: "Deck" });
        endpoints.createTeam.mockResolvedValue(team);
        const { result } = renderHook(useCreateTeam, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let created: unknown;
        await act(async () => {
            created = await result.current.mutateAsync({ orgId: "o1", name: "Deck" });
        });

        // then
        expect(created).toEqual(team);
    });

    it("hands back the plan's refusal of a second team and leaves the teams alone", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const refusal = makeUpgradeRequiredError();
        endpoints.createTeam.mockRejectedValue(refusal);
        const { result } = renderHook(useCreateTeam, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let thrown: unknown;
        await act(async () => {
            thrown = await result.current.mutateAsync({ orgId: "o1", name: "Deck" }).catch((error: unknown) => error);
        });

        // then
        expect(thrown).toBe(refusal);
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useDeleteTeam", () => {
    it("drops everything read under the deleted team instead of fetching it again", async () => {
        // given
        const qc = createTestQueryClient();
        const remove = vi.spyOn(qc, "removeQueries");
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const { result } = renderHook(useDeleteTeam, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync({ orgId: "o1", teamId: "t1" });
        });

        // then
        expect(remove).toHaveBeenCalledWith({ queryKey: queryKeys.teams.scope("t1") });
        expect(remove.mock.invocationCallOrder[0]).toBeLessThan(invalidate.mock.invocationCallOrder[0]);
    });
});
