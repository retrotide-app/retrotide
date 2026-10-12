import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, type MutationStateFixture } from "../../test-utils/fixtures";
import { providerWrapper } from "../../test-utils/render";
import { CARD_DELETE_UNDO_MS, useDeferredCardDelete } from "./useDeferredCardDelete";

const boards = vi.hoisted(() => ({ useDeleteCard: vi.fn() }));

vi.mock("../mutations/boards", () => boards);

const BOARD = "b1";

let removal: MutationStateFixture;

function renderDeferred() {
    return renderHook(() => useDeferredCardDelete(BOARD), { wrapper: providerWrapper() });
}

beforeEach(() => {
    vi.useFakeTimers();
    removal = makeMutationState();
    boards.useDeleteCard.mockReturnValue(removal);
});

afterEach(() => {
    vi.useRealTimers();
});

describe("useDeferredCardDelete", () => {
    it("hides a scheduled card at once and deletes it only when the undo window ends", async () => {
        // given
        const { result } = renderDeferred();

        // when
        act(() => result.current.schedule("k1"));
        const beforeExpiry = { hidden: result.current.hidden.has("k1"), waiting: result.current.waiting };
        await act(async () => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS - 1);
        });
        const sentEarly = removal.mutateAsync.mock.calls.length;
        await act(async () => {
            vi.advanceTimersByTime(1);
        });

        // then
        expect(boards.useDeleteCard).toHaveBeenCalledWith(BOARD);
        expect(beforeExpiry).toEqual({ hidden: true, waiting: ["k1"] });
        expect(sentEarly).toBe(0);
        expect(removal.mutateAsync).toHaveBeenCalledExactlyOnceWith({ cardId: "k1" });
        expect(result.current.waiting).toEqual([]);
        expect(result.current.hidden.has("k1")).toBe(true);
    });

    it("brings the card back and sends nothing when the delete is undone", () => {
        // given
        const { result } = renderDeferred();
        act(() => result.current.schedule("k1"));

        // when
        act(() => result.current.undo("k1"));
        act(() => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS);
        });

        // then
        expect(result.current.hidden.has("k1")).toBe(false);
        expect(result.current.waiting).toEqual([]);
        expect(removal.mutateAsync).not.toHaveBeenCalled();
        expect(removal.mutate).not.toHaveBeenCalled();
    });

    it("schedules a card only once, however often it is asked", async () => {
        // given
        const { result } = renderDeferred();

        // when
        act(() => result.current.schedule("k1"));
        act(() => result.current.schedule("k1"));
        await act(async () => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS);
        });

        // then
        expect(removal.mutateAsync).toHaveBeenCalledOnce();
    });

    it("keeps each card on its own timer, newest last", async () => {
        // given
        const { result } = renderDeferred();
        act(() => result.current.schedule("k1"));
        act(() => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS / 2);
        });

        // when
        act(() => result.current.schedule("k2"));
        const both = result.current.waiting;
        await act(async () => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS / 2);
        });

        // then
        expect(both).toEqual(["k1", "k2"]);
        expect(removal.mutateAsync).toHaveBeenCalledExactlyOnceWith({ cardId: "k1" });
        expect(result.current.waiting).toEqual(["k2"]);
    });

    it("shows the card again with the reason when the delete is refused", async () => {
        // given
        removal.mutateAsync.mockRejectedValue(makeApiError({ status: 409, message: "cards are locked on this board" }));
        const { result } = renderDeferred();
        act(() => result.current.schedule("k1"));

        // when
        await act(async () => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS);
        });

        // then
        expect(result.current.hidden.has("k1")).toBe(false);
        expect(result.current.error?.message).toBe("cards are locked on this board");
    });

    it("clears a refusal once dismissed", async () => {
        // given
        removal.mutateAsync.mockRejectedValue(makeApiError({ status: 404, message: "card not found" }));
        const { result } = renderDeferred();
        act(() => result.current.schedule("k1"));
        await act(async () => {
            vi.advanceTimersByTime(CARD_DELETE_UNDO_MS);
        });

        // when
        act(() => result.current.dismissError());

        // then
        expect(result.current.error).toBeNull();
    });

    it("sends every waiting delete at once when the board goes away", () => {
        // given
        const { result, unmount } = renderDeferred();
        act(() => result.current.schedule("k1"));
        act(() => result.current.schedule("k2"));

        // when
        unmount();
        vi.advanceTimersByTime(CARD_DELETE_UNDO_MS);

        // then
        expect(removal.mutate).toHaveBeenCalledTimes(2);
        expect(removal.mutate).toHaveBeenCalledWith({ cardId: "k1" });
        expect(removal.mutate).toHaveBeenCalledWith({ cardId: "k2" });
        expect(removal.mutateAsync).not.toHaveBeenCalled();
    });
});
