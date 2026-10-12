import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PendingCommand } from "../../domain/board/optimistic";
import { makeBoardSnapshot, makeCard, makeQueryState } from "../../test-utils/fixtures";
import type { BoardSnapshot } from "../../types/api";
import { useBoardView } from "./useBoardView";

const { useBoard } = vi.hoisted(() => ({ useBoard: vi.fn() }));

const { usePendingBoardCommands } = vi.hoisted(() => ({ usePendingBoardCommands: vi.fn() }));

const { useBoardSync } = vi.hoisted(() => ({ useBoardSync: vi.fn() }));

vi.mock("../queries/boards", () => ({ useBoard }));
vi.mock("../mutations/boards", () => ({ usePendingBoardCommands }));
vi.mock("../../api/realtime/sync/useBoardSync", () => ({ useBoardSync }));

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const confirmed = makeBoardSnapshot({
    version: 5,
    cards: [makeCard({ id: "k1", column_id: START, position: "a0" })],
});

const moveToStop: PendingCommand = {
    change: { kind: "card-move", cardId: "k1", drop: { column_id: STOP, after_card_id: null } },
    answeredAt: null,
};

beforeEach(() => {
    vi.clearAllMocks();
    useBoard.mockReturnValue(makeQueryState<BoardSnapshot>({ data: confirmed }));
    usePendingBoardCommands.mockReturnValue([]);
    useBoardSync.mockReturnValue({ state: "live" });
});

describe("useBoardView", () => {
    it("reads the board, its pending commands and its live sync for the one board it was given", () => {
        // when
        renderHook(() => useBoardView("b1"));

        // then
        expect(useBoard).toHaveBeenCalledWith("b1");
        expect(usePendingBoardCommands).toHaveBeenCalledWith("b1");
        expect(useBoardSync).toHaveBeenCalledWith("b1");
    });

    it("hands back how the board's live sync stands", () => {
        // given
        useBoardSync.mockReturnValue({ state: "refused", reason: "removed", revoked: true });

        // when
        const { result } = renderHook(() => useBoardView("b1"));

        // then
        expect(result.current.sync).toEqual({ state: "refused", reason: "removed", revoked: true });
    });

    it("shows the confirmed board as it is when nothing is pending", () => {
        // when
        const { result } = renderHook(() => useBoardView("b1"));

        // then
        expect(result.current.data).toBe(confirmed);
    });

    it("lays a pending move over the confirmed board", () => {
        // given
        usePendingBoardCommands.mockReturnValue([moveToStop]);

        // when
        const { result } = renderHook(() => useBoardView("b1"));

        // then
        expect(result.current.data?.cards[0].column_id).toBe(STOP);
        expect(result.current.data?.version).toBe(5);
    });

    it("stops showing an answered move once the confirmed board has reached its version", () => {
        // given
        usePendingBoardCommands.mockReturnValue([{ ...moveToStop, answeredAt: 5 }]);

        // when
        const { result } = renderHook(() => useBoardView("b1"));

        // then
        expect(result.current.data?.cards[0].column_id).toBe(START);
    });

    it("has no board to show while none is loaded, and passes the loading state through", () => {
        // given
        useBoard.mockReturnValue(makeQueryState<BoardSnapshot>({ loading: true }));
        usePendingBoardCommands.mockReturnValue([moveToStop]);

        // when
        const { result } = renderHook(() => useBoardView("b1"));

        // then
        expect(result.current.data).toBeUndefined();
        expect(result.current.loading).toBe(true);
    });

    it("keeps the same board between renders when nothing changed", () => {
        // given
        const pending = [moveToStop];
        usePendingBoardCommands.mockReturnValue(pending);
        const { result, rerender } = renderHook(() => useBoardView("b1"));
        const first = result.current.data;

        // when
        rerender();

        // then
        expect(result.current.data).toBe(first);
    });
});
