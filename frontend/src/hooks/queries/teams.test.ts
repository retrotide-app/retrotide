import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeTeam, makeTeamMember } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useTeam, useTeamMembers, useTeams } from "./teams";

const endpoints = vi.hoisted(() => ({
    getTeam: vi.fn(),
    listTeamMembers: vi.fn(),
    listTeams: vi.fn(),
}));

vi.mock("../../api/endpoints/teams", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.listTeams.mockResolvedValue({ teams: [] });
    endpoints.getTeam.mockResolvedValue(makeTeam());
    endpoints.listTeamMembers.mockResolvedValue({ members: [] });
});

describe("useTeams", () => {
    it("reads the organisation's teams under its teams key", async () => {
        // given
        const listing = { teams: [makeTeam(), makeTeam({ id: "t2", archived_at: "2026-09-20T09:00:00Z" })] };
        endpoints.listTeams.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useTeams("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listTeams).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.organisations.teams("o1"))).toEqual(listing);
    });

    it("asks nothing without an organisation id", () => {
        // when
        const { result } = setup(() => useTeams(""));

        // then
        expect(endpoints.listTeams).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});

describe("useTeam", () => {
    it("reads one team under its detail key", async () => {
        // given
        const team = makeTeam({ id: "t1" });
        endpoints.getTeam.mockResolvedValue(team);

        // when
        const { result, queryClient } = setup(() => useTeam("t1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(team));
        expect(endpoints.getTeam).toHaveBeenCalledWith("t1");
        expect(queryClient.getQueryData(queryKeys.teams.detail("t1"))).toEqual(team);
    });

    it("hands back the error when the team could not be read", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "team not found" });
        endpoints.getTeam.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useTeam("t1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });

    it("asks nothing without a team id", () => {
        // when
        setup(() => useTeam(""));

        // then
        expect(endpoints.getTeam).not.toHaveBeenCalled();
    });
});

describe("useTeamMembers", () => {
    it("reads the team's members under its members key", async () => {
        // given
        const listing = { members: [makeTeamMember({ role: "lead" })] };
        endpoints.listTeamMembers.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useTeamMembers("t1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listTeamMembers).toHaveBeenCalledWith("t1");
        expect(queryClient.getQueryData(queryKeys.teams.members("t1"))).toEqual(listing);
    });

    it("asks nothing without a team id", () => {
        // when
        setup(() => useTeamMembers(""));

        // then
        expect(endpoints.listTeamMembers).not.toHaveBeenCalled();
    });
});
