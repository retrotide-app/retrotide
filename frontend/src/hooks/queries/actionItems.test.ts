import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeActionItemList, makeApiError, makeMention, makePerson } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useBoardPeople, useMentions, useOrganisationActionItems } from "./actionItems";

const endpoints = vi.hoisted(() => ({
    listBoardPeople: vi.fn(),
    listOrganisationActionItems: vi.fn(),
    listMentions: vi.fn(),
}));

vi.mock("../../api/endpoints/actionItems", () => endpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
    }
});

describe("useBoardPeople", () => {
    it("reads who can be named on the board under its people key", async () => {
        // given
        const people = { people: [makePerson()] };
        endpoints.listBoardPeople.mockResolvedValue(people);

        // when
        const { result, queryClient } = setup(() => useBoardPeople("b1", true));

        // then
        await waitFor(() => expect(result.current.data).toEqual(people));
        expect(endpoints.listBoardPeople).toHaveBeenCalledWith("b1");
        expect(queryClient.getQueryData(queryKeys.boardPeople("b1"))).toEqual(people);
    });

    it("hands back the refusal a guest gets, so the picker can stay hidden", async () => {
        // given
        const refusal = makeApiError({ status: 403, message: "members only", code: "members_only" });
        endpoints.listBoardPeople.mockRejectedValue(refusal);

        // when
        const { result } = setup(() => useBoardPeople("b1", true));

        // then
        await waitFor(() => expect(result.current.error).toBe(refusal));
    });

    it("asks nothing until it is wanted", () => {
        // when
        setup(() => useBoardPeople("b1", false));

        // then
        expect(endpoints.listBoardPeople).not.toHaveBeenCalled();
    });
});

describe("useOrganisationActionItems", () => {
    it("reads the page the filters ask for under that organisation's key", async () => {
        // given
        const listing = makeActionItemList();
        endpoints.listOrganisationActionItems.mockResolvedValue(listing);
        const query = { status: "open" as const, assignee: "me" };

        // when
        const { result, queryClient } = setup(() => useOrganisationActionItems("o1", query));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listOrganisationActionItems).toHaveBeenCalledWith("o1", query);
        expect(queryClient.getQueryData(queryKeys.actionItems.list("o1", query))).toEqual(listing);
    });

    it("asks nothing without an organisation", () => {
        // when
        setup(() => useOrganisationActionItems("", {}));

        // then
        expect(endpoints.listOrganisationActionItems).not.toHaveBeenCalled();
    });
});

describe("useMentions", () => {
    it("reads the account's mentions under the auth root", async () => {
        // given
        const mentions = { mentions: [makeMention()] };
        endpoints.listMentions.mockResolvedValue(mentions);

        // when
        const { result, queryClient } = setup(useMentions);

        // then
        await waitFor(() => expect(result.current.data).toEqual(mentions));
        expect(queryClient.getQueryData(queryKeys.auth.mentions())).toEqual(mentions);
    });
});
