import type { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardSnapshot, makePreviousAction } from "../../test-utils/fixtures";
import { createTestQueryClient } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { queryKeys } from "../queryKeys";
import { boardSnapshotQuery, catchUpBoard, refreshPreviousActions } from "./boardSnapshot";

const { getBoard } = vi.hoisted(() => ({ getBoard: vi.fn() }));

vi.mock("../endpoints/boards", () => ({ getBoard }));

const STATE = queryKeys.boardLive.state("b1");

let qc: QueryClient;

function held(): BoardSnapshot | undefined {
    return qc.getQueryData<BoardSnapshot>(STATE);
}

beforeEach(() => {
    getBoard.mockReset();
    qc = createTestQueryClient();
    qc.setQueryDefaults(STATE, { gcTime: Infinity });
});

describe("boardSnapshotQuery", () => {
    it("reads the whole board under its live state key when nothing is held", async () => {
        // given
        const snapshot = makeBoardSnapshot({ board_id: "b1", version: 3 });
        getBoard.mockResolvedValue(snapshot);

        // when
        await qc.query(boardSnapshotQuery(qc, "b1"));

        // then
        expect(getBoard).toHaveBeenCalledWith("b1", undefined, undefined);
        expect(held()).toEqual(snapshot);
    });

    it("sends the version it holds, and keeps the board when the server says nothing changed", async () => {
        // given
        const snapshot = makeBoardSnapshot({ board_id: "b1", version: 3 });
        qc.setQueryData(STATE, snapshot);
        getBoard.mockResolvedValue(undefined);

        // when
        await qc.query({ ...boardSnapshotQuery(qc, "b1"), staleTime: 0 });

        // then
        expect(getBoard).toHaveBeenCalledWith("b1", 3, undefined);
        expect(held()).toBe(snapshot);
    });

    it("says so when the server answers unchanged before any board is held", async () => {
        // given
        getBoard.mockResolvedValue(undefined);

        // when
        const attempt = qc.query(boardSnapshotQuery(qc, "b1"));

        // then
        await expect(attempt).rejects.toThrow("answered unchanged");
    });
});

describe("catchUpBoard", () => {
    it("asks for the board as a reconnect, sending the version it holds, and takes a newer one", async () => {
        // given
        qc.setQueryData(STATE, makeBoardSnapshot({ board_id: "b1", version: 3 }));
        const newer = makeBoardSnapshot({ board_id: "b1", version: 7, cards: [] });
        getBoard.mockResolvedValue(newer);

        // when
        await catchUpBoard(qc, "b1");

        // then
        expect(getBoard).toHaveBeenCalledWith("b1", 3, "reconnect");
        expect(held()).toEqual(newer);
    });

    it("fetches even when the board held is fresh, because only the server knows what changed", async () => {
        // given
        qc.setQueryData(STATE, makeBoardSnapshot({ board_id: "b1", version: 3 }));
        getBoard.mockResolvedValue(undefined);

        // when
        await catchUpBoard(qc, "b1");

        // then
        expect(getBoard).toHaveBeenCalledTimes(1);
    });

    it("keeps a newer board than the answer, such as one live frames have already moved on", async () => {
        // given
        const newest = makeBoardSnapshot({ board_id: "b1", version: 9 });
        qc.setQueryData(STATE, newest);
        getBoard.mockResolvedValue(makeBoardSnapshot({ board_id: "b1", version: 8, cards: [] }));

        // when
        await catchUpBoard(qc, "b1");

        // then
        expect(held()).toBe(newest);
    });

    it("hands back the failure so the caller can try again, and leaves the board it holds", async () => {
        // given
        const snapshot = makeBoardSnapshot({ board_id: "b1", version: 3 });
        qc.setQueryData(STATE, snapshot);
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        getBoard.mockRejectedValue(failure);

        // when
        const attempt = catchUpBoard(qc, "b1");

        // then
        await expect(attempt).rejects.toBe(failure);
        expect(held()).toBe(snapshot);
    });
});

describe("refreshPreviousActions", () => {
    it("reads the whole board without a version, because another board changed the team's items", async () => {
        // given
        qc.setQueryData(STATE, makeBoardSnapshot({ board_id: "b1", version: 3 }));
        const fresh = makeBoardSnapshot({
            board_id: "b1",
            version: 3,
            previous_actions: [makePreviousAction({ status: "done", review: "done" })],
        });
        getBoard.mockResolvedValue(fresh);

        // when
        await refreshPreviousActions(qc, "b1");

        // then
        expect(getBoard).toHaveBeenCalledWith("b1");
        expect(held()?.previous_actions).toEqual(fresh.previous_actions);
    });

    it("reads again when live frames have moved the board past the answer, so a stale list never undoes a review", async () => {
        // given
        const newest = makeBoardSnapshot({ board_id: "b1", version: 9, cards: [] });
        qc.setQueryData(STATE, newest);
        const stale = [makePreviousAction()];
        const current = [makePreviousAction({ status: "done", review: "done" })];
        getBoard
            .mockResolvedValueOnce(makeBoardSnapshot({ board_id: "b1", version: 8, previous_actions: stale }))
            .mockResolvedValueOnce(makeBoardSnapshot({ board_id: "b1", version: 9, previous_actions: current }));

        // when
        await refreshPreviousActions(qc, "b1");

        // then
        expect(getBoard).toHaveBeenCalledTimes(2);
        expect(held()).toEqual({ ...newest, previous_actions: current });
    });

    it("takes a newer board whole", async () => {
        // given
        qc.setQueryData(STATE, makeBoardSnapshot({ board_id: "b1", version: 3 }));
        const newer = makeBoardSnapshot({ board_id: "b1", version: 5, cards: [], previous_actions: [] });
        getBoard.mockResolvedValue(newer);

        // when
        await refreshPreviousActions(qc, "b1");

        // then
        expect(held()).toEqual(newer);
    });

    it("leaves a board it no longer holds alone", async () => {
        // given
        getBoard.mockResolvedValue(makeBoardSnapshot({ board_id: "b1", version: 3 }));

        // when
        await refreshPreviousActions(qc, "b1");

        // then
        expect(held()).toBeUndefined();
    });

    it("hands back the failure and keeps the board it holds", async () => {
        // given
        const snapshot = makeBoardSnapshot({ board_id: "b1", version: 3 });
        qc.setQueryData(STATE, snapshot);
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        getBoard.mockRejectedValue(failure);

        // when
        const attempt = refreshPreviousActions(qc, "b1");

        // then
        await expect(attempt).rejects.toBe(failure);
        expect(held()).toBe(snapshot);
    });
});
