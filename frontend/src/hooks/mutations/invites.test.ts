import type { QueryKey } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import {
    makeApiError,
    makeCreatedInvite,
    makeInviteRegistration,
    makeJoinedOrganisation,
    makeUpgradeRequiredError,
} from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import {
    useAcceptInvite,
    useCreateInvite,
    useRegisterThroughInvite,
    useResendInvite,
    useRevokeInvite,
} from "./invites";

const endpoints = vi.hoisted(() => ({
    acceptInvite: vi.fn(),
    createInvite: vi.fn(),
    registerThroughInvite: vi.fn(),
    resendInvite: vi.fn(),
    revokeInvite: vi.fn(),
}));

vi.mock("../../api/endpoints/invites", () => endpoints);

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

const HARBOUR_INVITES = queryKeys.organisations.invites("o1");

const cases: MutationCase[] = [
    {
        name: "useCreateInvite creates the invite and refreshes the organisation's invites",
        use: useCreateInvite,
        variables: { orgId: "o1", email: "kit@example.test", role: "member", team_id: "t1" },
        endpoint: endpoints.createInvite,
        request: ["o1", { email: "kit@example.test", role: "member", team_id: "t1" }],
        invalidated: [HARBOUR_INVITES],
    },
    {
        name: "useResendInvite sends the invite again and refreshes the organisation's invites",
        use: useResendInvite,
        variables: { orgId: "o1", inviteId: "i1" },
        endpoint: endpoints.resendInvite,
        request: ["o1", "i1"],
        invalidated: [HARBOUR_INVITES],
    },
    {
        name: "useRevokeInvite revokes the invite and refreshes the organisation's invites",
        use: useRevokeInvite,
        variables: { orgId: "o1", inviteId: "i1" },
        endpoint: endpoints.revokeInvite,
        request: ["o1", "i1"],
        invalidated: [HARBOUR_INVITES],
    },
    {
        name: "useAcceptInvite joins the organisation and refreshes the user's organisations",
        use: useAcceptInvite,
        variables: { token: "invite-1" },
        endpoint: endpoints.acceptInvite,
        request: [{ token: "invite-1" }],
        invalidated: [queryKeys.organisations.list()],
    },
];

const outdated: OutdatedCase[] = [
    {
        name: "useResendInvite, whose invite was used or revoked meanwhile",
        use: useResendInvite,
        variables: { orgId: "o1", inviteId: "i1" },
        endpoint: endpoints.resendInvite,
        status: 404,
        invalidated: [HARBOUR_INVITES],
    },
    {
        name: "useRevokeInvite, whose invite is already gone",
        use: useRevokeInvite,
        variables: { orgId: "o1", inviteId: "i1" },
        endpoint: endpoints.revokeInvite,
        status: 404,
        invalidated: [HARBOUR_INVITES],
    },
    {
        name: "useAcceptInvite, for someone who is already a member",
        use: useAcceptInvite,
        variables: { token: "invite-1" },
        endpoint: endpoints.acceptInvite,
        status: 409,
        invalidated: [queryKeys.organisations.list()],
    },
];

const NOT_SENT_MESSAGE = "the invite was saved, but its email was not sent, so resend it";

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockResolvedValue(undefined);
    }
});

describe("invite mutations", () => {
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
        endpoint.mockRejectedValue(makeApiError({ status: 403, message: "inviting an admin needs the owner" }));
        const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await expect(result.current.mutateAsync(variables as never)).rejects.toThrow(
                "inviting an admin needs the owner",
            );
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });

    it.each(cases)(
        "refreshes after a 500, which the server also answers for a saved change: $name",
        async ({ use, variables, endpoint, invalidated }) => {
            // given
            const qc = createTestQueryClient();
            const invalidate = vi.spyOn(qc, "invalidateQueries");
            endpoint.mockRejectedValue(makeApiError({ status: 500, message: NOT_SENT_MESSAGE }));
            const { result } = renderHook(use, { wrapper: providerWrapper({ queryClient: qc }) });

            // when
            await act(async () => {
                await expect(result.current.mutateAsync(variables as never)).rejects.toThrow(NOT_SENT_MESSAGE);
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

describe("useCreateInvite", () => {
    it("hands back a link invite's address, which this answer alone carries", async () => {
        // given
        const qc = createTestQueryClient();
        const created = makeCreatedInvite({ email: undefined, url: "https://retrotide.test/invite#token-1" });
        endpoints.createInvite.mockResolvedValue(created);
        const { result } = renderHook(useCreateInvite, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let answer: unknown;
        await act(async () => {
            answer = await result.current.mutateAsync({ orgId: "o1", role: "member" });
        });

        // then
        expect(answer).toEqual(created);
    });
});

describe("useAcceptInvite", () => {
    it("hands back the organisation the user joined", async () => {
        // given
        const qc = createTestQueryClient();
        const joined = makeJoinedOrganisation({ team_id: "t1" });
        endpoints.acceptInvite.mockResolvedValue(joined);
        const { result } = renderHook(useAcceptInvite, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let answer: unknown;
        await act(async () => {
            answer = await result.current.mutateAsync({ token: "invite-1" });
        });

        // then
        expect(answer).toEqual(joined);
    });

    it("hands back the plan's refusal when the organisation is full, and leaves the list alone", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const refusal = makeUpgradeRequiredError({ limit: "members", allowed: 10, current: 10 });
        endpoints.acceptInvite.mockRejectedValue(refusal);
        const { result } = renderHook(useAcceptInvite, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let thrown: unknown;
        await act(async () => {
            thrown = await result.current.mutateAsync({ token: "invite-1" }).catch((error: unknown) => error);
        });

        // then
        expect(thrown).toBe(refusal);
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useRegisterThroughInvite", () => {
    it("creates the account through the invite, hands back what it joined and touches no cache", async () => {
        // given
        const qc = createTestQueryClient();
        const invalidate = vi.spyOn(qc, "invalidateQueries");
        const registration = makeInviteRegistration({ verification_required: true });
        endpoints.registerThroughInvite.mockResolvedValue(registration);
        const body = { token: "invite-1", email: "", display_name: "Kit", password: "harbour-lights" };
        const { result } = renderHook(useRegisterThroughInvite, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let answer: unknown;
        await act(async () => {
            answer = await result.current.mutateAsync(body);
        });

        // then
        expect(endpoints.registerThroughInvite).toHaveBeenCalledWith(body, expect.anything());
        expect(answer).toEqual(registration);
        expect(invalidate).not.toHaveBeenCalled();
    });
});
