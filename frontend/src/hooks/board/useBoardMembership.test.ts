import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardPeopleResponse } from "../../types/api";
import { makeApiError, makePerson, makeQueryState } from "../../test-utils/fixtures";
import { useBoardMembership } from "./useBoardMembership";

const { useBoardPeople } = vi.hoisted(() => ({ useBoardPeople: vi.fn() }));

vi.mock("../queries/actionItems", () => ({ useBoardPeople }));

beforeEach(() => {
    useBoardPeople.mockReset();
});

describe("useBoardMembership", () => {
    it("treats a viewer who can read the people as an organisation member", () => {
        // given
        const people = [makePerson()];
        useBoardPeople.mockReturnValue(makeQueryState<BoardPeopleResponse>({ data: { people } }));

        // when
        const { result } = renderHook(() => useBoardMembership("b1", true));

        // then
        expect(useBoardPeople).toHaveBeenCalledWith("b1", true);
        expect(result.current.member).toBe(true);
        expect(result.current.people).toEqual(people);
        expect(result.current.failure).toBeNull();
    });

    it("treats a members only refusal as a guest, with no people and nothing to report", () => {
        // given
        const refusal = makeApiError({ status: 403, message: "members only", code: "members_only" });
        useBoardPeople.mockReturnValue(makeQueryState<BoardPeopleResponse>({ error: refusal }));

        // when
        const { result } = renderHook(() => useBoardMembership("b1", true));

        // then
        expect(result.current.member).toBe(false);
        expect(result.current.people).toBeNull();
        expect(result.current.failure).toBeNull();
    });

    it("reports any other failure rather than treating the viewer as a guest silently", () => {
        // given
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        useBoardPeople.mockReturnValue(makeQueryState<BoardPeopleResponse>({ error: failure }));

        // when
        const { result } = renderHook(() => useBoardMembership("b1", true));

        // then
        expect(result.current.member).toBe(false);
        expect(result.current.failure).toBe(failure);
    });

    it("names nobody while the people are loading", () => {
        // given
        useBoardPeople.mockReturnValue(makeQueryState<BoardPeopleResponse>({ loading: true }));

        // when
        const { result } = renderHook(() => useBoardMembership("b1", true));

        // then
        expect(result.current.member).toBe(false);
        expect(result.current.people).toBeNull();
    });
});
