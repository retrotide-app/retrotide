import { act, renderHook } from "@testing-library/react";
import type { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeBoardSnapshot, makeBoardView, makeCard } from "../../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../../test-utils/render";
import { emitRealtimeEvent, makeWSHarness, type WSHarness } from "../../../test-utils/ws";
import type { BoardEvent, BoardSnapshot } from "../../../types/api";
import { queryKeys } from "../../queryKeys";
import type * as OutboundModule from "../outbound";
import { useBoardSync } from "./useBoardSync";

const holder = vi.hoisted(() => ({ ws: null as unknown as WSHarness }));

const { catchUp, refresh, reportClientError } = vi.hoisted(() => ({
    catchUp: vi.fn(),
    refresh: vi.fn(),
    reportClientError: vi.fn(),
}));

vi.mock("../pipeline", () => ({
    ensureRealtimePipeline: () => {},
    getRealtimeEpoch: () => holder.ws.getEpoch(),
    getRealtimeConnected: () => holder.ws.getConnected(),
    subscribeRealtimeEpoch: (listener: () => void) => holder.ws.subscribeEpoch(listener),
}));

vi.mock("../outbound", async importOriginal => {
    const actual = await importOriginal<typeof OutboundModule>();

    return { ...actual, sendRealtime: (command: OutboundModule.RealtimeCommand) => holder.ws.sendRealtime(command) };
});

vi.mock("../../cache/boardSnapshot", () => ({ catchUpBoard: catchUp, refreshPreviousActions: refresh }));

vi.mock("../../telemetry", () => ({ reportClientError }));

const STATE = queryKeys.boardLive.state(BOARD_ID);

const OTHER_BOARD = "00000000-0000-4000-8000-0000000000b2";

const BOARD_REF = { board_id: BOARD_ID } as const;

const START = "00000000-0000-4000-8000-000000000c01";

let queryClient: QueryClient;

function hold(version: number, overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    const snapshot = makeBoardSnapshot({ version, cards: [], ...overrides });
    queryClient.setQueryData(STATE, snapshot);

    return snapshot;
}

function held(): BoardSnapshot | undefined {
    return queryClient.getQueryData<BoardSnapshot>(STATE);
}

function cardAdded(version: number, id: string): BoardEvent {
    return { type: "card_upserted", version, card: makeCard({ id, column_id: START }) };
}

function frame(from: number, to: number, events: BoardEvent[], boardId = BOARD_ID) {
    return { type: "board_events" as const, data: { board_id: boardId, from, to, events } };
}

function joined(version: number, boardId = BOARD_ID) {
    return {
        type: "board_joined" as const,
        data: { board_id: boardId, version, participant_id: null, role: "participant" },
    };
}

function commandsSent(type: OutboundModule.RealtimeCommandName): unknown[] {
    return holder.ws.sendRealtime.mock.calls
        .map(([command]) => command)
        .filter(command => command.type === type)
        .map(command => command.data);
}

function catchUpTo(snapshot: Partial<BoardSnapshot> & { version: number }): void {
    catchUp.mockImplementationOnce(async () => {
        hold(snapshot.version, snapshot);
    });
}

const MICROTASK_TURNS = 10;

async function settle(): Promise<void> {
    await act(async () => {
        for (let turn = 0; turn < MICROTASK_TURNS; turn += 1) {
            await Promise.resolve();
        }
    });
}

function mount() {
    return renderHook(() => useBoardSync(BOARD_ID), { wrapper: providerWrapper({ queryClient }) });
}

function mountOn(boardId: string) {
    return renderHook((props: { boardId: string }) => useBoardSync(props.boardId), {
        wrapper: providerWrapper({ queryClient }),
        initialProps: { boardId },
    });
}

function mountOnOpenSocket() {
    holder.ws.reconnect();

    return mount();
}

beforeEach(() => {
    holder.ws = makeWSHarness();
    queryClient = createTestQueryClient();
    queryClient.setQueryDefaults(STATE, { gcTime: Infinity });
    catchUp.mockReset();
    catchUp.mockResolvedValue(undefined);
    refresh.mockReset();
    refresh.mockResolvedValue(undefined);
    reportClientError.mockReset();
});

afterEach(() => {
    vi.useRealTimers();
});

describe("useBoardSync joining the board", () => {
    it("waits for the socket to open before joining, and says it is connecting", () => {
        // when
        const { result } = mount();

        // then
        expect(commandsSent("board_join")).toEqual([]);
        expect(result.current).toEqual({ state: "connecting" });
    });

    it("joins the board once the socket is open", () => {
        // when
        mountOnOpenSocket();

        // then
        expect(commandsSent("board_join")).toEqual([BOARD_REF]);
    });

    it("joins as soon as a socket that opens later is up", () => {
        // given
        mount();

        // when
        holder.ws.reconnect();

        // then
        expect(commandsSent("board_join")).toEqual([BOARD_REF]);
    });

    it("leaves the board when it closes", () => {
        // given
        const { unmount } = mountOnOpenSocket();

        // when
        unmount();

        // then
        expect(commandsSent("board_leave")).toEqual([BOARD_REF]);
    });

    it("sends the join again when no answer arrives within five seconds", () => {
        // given
        vi.useFakeTimers();
        mountOnOpenSocket();

        // when
        act(() => {
            vi.advanceTimersByTime(5_000);
        });

        // then
        expect(commandsSent("board_join")).toEqual([BOARD_REF, BOARD_REF]);
    });

    it("stops resending the join once the board has answered", () => {
        // given
        vi.useFakeTimers();
        hold(10);
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        act(() => {
            vi.advanceTimersByTime(15_000);
        });

        // then
        expect(commandsSent("board_join")).toEqual([BOARD_REF]);
    });

    it("goes live at once when the board held is already at the joined version", () => {
        // given
        hold(10);
        const { result } = mountOnOpenSocket();

        // when
        emitRealtimeEvent(joined(10));

        // then
        expect(result.current).toEqual({ state: "live" });
        expect(catchUp).not.toHaveBeenCalled();
    });

    it("fetches the board after the join when the one held is older, then applies what arrived meanwhile", async () => {
        // given
        hold(7);
        catchUpTo({ version: 10 });
        const { result } = mountOnOpenSocket();

        // when
        emitRealtimeEvent(joined(10));
        emitRealtimeEvent(frame(11, 11, [cardAdded(11, "k-new")]));
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledExactlyOnceWith(queryClient, BOARD_ID);
        expect(held()?.version).toBe(11);
        expect(held()?.cards.map(card => card.id)).toEqual(["k-new"]);
        expect(result.current).toEqual({ state: "live" });
    });

    it("fetches again when the board fetched was read before the join and is still older", async () => {
        // given
        catchUpTo({ version: 9 });
        catchUpTo({ version: 10 });
        const { result } = mountOnOpenSocket();

        // when
        emitRealtimeEvent(joined(10));
        await settle();
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledTimes(2);
        expect(result.current).toEqual({ state: "live" });
    });

    it("ignores the answer for another board", () => {
        // given
        hold(10);
        const { result } = mountOnOpenSocket();

        // when
        emitRealtimeEvent(joined(10, OTHER_BOARD));

        // then
        expect(result.current).toEqual({ state: "connecting" });
    });
});

describe("useBoardSync while live", () => {
    function goLive(version: number) {
        hold(version);
        const rendered = mountOnOpenSocket();
        emitRealtimeEvent(joined(version));

        return rendered;
    }

    it("applies a frame to the board's live state", () => {
        // given
        goLive(10);

        // when
        emitRealtimeEvent(frame(11, 12, [cardAdded(11, "k1"), { type: "card_deleted", version: 12, card_id: "k1" }]));

        // then
        expect(held()?.version).toBe(12);
        expect(held()?.cards).toEqual([]);
    });

    it("applies the frame for its own command once, however often it arrives", () => {
        // given
        goLive(10);
        const echo = frame(11, 11, [cardAdded(11, "k-mine")]);

        // when
        emitRealtimeEvent(echo);
        emitRealtimeEvent(echo);

        // then
        expect(held()?.cards.map(card => card.id)).toEqual(["k-mine"]);
        expect(held()?.version).toBe(11);
    });

    it("drops a frame the board has already reached through a refetch", () => {
        // given
        goLive(10);
        const refetched = hold(12);

        // when
        emitRealtimeEvent(frame(11, 12, [cardAdded(11, "k1"), cardAdded(12, "k2")]));

        // then
        expect(held()).toEqual(refetched);
        expect(held()?.cards).toEqual([]);
    });

    it("ignores frames for another board on the same socket", () => {
        // given
        goLive(10);
        const before = held();

        // when
        emitRealtimeEvent(frame(11, 11, [cardAdded(11, "k1")], OTHER_BOARD));

        // then
        expect(held()).toBe(before);
    });

    it("refetches once after a gap, however many frames follow, then applies them in order", async () => {
        // given
        const { result } = goLive(10);
        catchUpTo({ version: 12 });

        // when
        emitRealtimeEvent(frame(13, 13, [cardAdded(13, "k13")]));
        emitRealtimeEvent(frame(14, 14, [cardAdded(14, "k14")]));

        // then
        expect(catchUp).toHaveBeenCalledTimes(1);
        expect(result.current).toEqual({ state: "connecting" });

        // when
        await settle();

        // then
        expect(held()?.cards.map(card => card.id)).toEqual(["k13", "k14"]);
        expect(held()?.version).toBe(14);
        expect(result.current).toEqual({ state: "live" });
    });

    it("refetches on a resync event", async () => {
        // given
        goLive(10);
        catchUpTo({ version: 11 });

        // when
        emitRealtimeEvent(frame(11, 11, [{ type: "resync", version: 11 }]));
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledTimes(1);
        expect(held()?.version).toBe(11);
    });

    it("tries the refetch again when it fails, at once and then after a pause", async () => {
        // given
        vi.useFakeTimers();
        goLive(10);
        catchUp.mockRejectedValueOnce(new Error("service unavailable")).mockRejectedValueOnce(new Error("again"));
        catchUpTo({ version: 12 });

        // when
        emitRealtimeEvent(frame(13, 13, [cardAdded(13, "k13")]));
        await settle();
        act(() => {
            vi.advanceTimersByTime(5_000);
        });
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledTimes(3);
        expect(held()?.version).toBe(13);
    });
});

describe("useBoardSync after a network cut", () => {
    it("says it is connecting as soon as the socket closes, before any reopen", () => {
        // given
        hold(10);
        const { result } = mountOnOpenSocket();
        emitRealtimeEvent(joined(10));
        expect(result.current).toEqual({ state: "live" });

        // when
        holder.ws.disconnect();

        // then
        expect(result.current).toEqual({ state: "connecting" });
    });

    it("keeps a refusal on screen while the socket is closed", () => {
        // given
        const { result } = mountOnOpenSocket();
        emitRealtimeEvent({ type: "board_access_revoked", data: { ...BOARD_REF, reason: "removed" } });

        // when
        holder.ws.disconnect();

        // then
        expect(result.current).toEqual({ state: "refused", reason: "removed", revoked: true });
    });

    it("joins again on the new socket and catches up from the board it holds", async () => {
        // given
        hold(10);
        const { result } = mountOnOpenSocket();
        emitRealtimeEvent(joined(10));
        catchUpTo({ version: 15 });

        // when
        holder.ws.reconnect();

        // then
        expect(result.current).toEqual({ state: "connecting" });
        expect(commandsSent("board_join")).toEqual([BOARD_REF, BOARD_REF]);

        // when
        emitRealtimeEvent(joined(15));
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledTimes(1);
        expect(held()?.version).toBe(15);
        expect(result.current).toEqual({ state: "live" });
    });

    it("forgets frames buffered on the old socket, because the new join starts afresh", async () => {
        // given
        hold(10);
        mountOnOpenSocket();
        emitRealtimeEvent(frame(11, 11, [cardAdded(11, "stale")]));

        // when
        holder.ws.reconnect();
        emitRealtimeEvent(joined(10));

        // then
        expect(held()?.cards).toEqual([]);
    });
});

describe("useBoardSync refusals", () => {
    it("stops and says why when the board refuses the join", () => {
        // given
        vi.useFakeTimers();
        const { result } = mountOnOpenSocket();

        // when
        emitRealtimeEvent({ type: "board_join_rejected", data: { ...BOARD_REF, reason: "removed" } });
        act(() => {
            vi.advanceTimersByTime(15_000);
        });

        // then
        expect(result.current).toEqual({ state: "refused", reason: "removed", revoked: false });
        expect(commandsSent("board_join")).toEqual([BOARD_REF]);
    });

    it("stops applying frames once access is revoked", () => {
        // given
        hold(10);
        const { result } = mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent({ type: "board_access_revoked", data: { ...BOARD_REF, reason: "not_participant" } });
        emitRealtimeEvent(frame(11, 11, [cardAdded(11, "k1")]));

        // then
        expect(result.current).toEqual({ state: "refused", reason: "not_participant", revoked: true });
        expect(held()?.version).toBe(10);
    });

    it("keeps the refusal when a refetch started earlier finishes afterwards", async () => {
        // given
        hold(7);
        const { result } = mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent({ type: "board_access_revoked", data: { ...BOARD_REF, reason: "removed" } });
        await settle();

        // then
        expect(result.current).toEqual({ state: "refused", reason: "removed", revoked: true });
    });

    it("never fetches again once access is revoked while a failing refetch is in flight", async () => {
        // given
        vi.useFakeTimers();
        catchUp.mockRejectedValue(new Error("forbidden"));
        hold(7);
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent({ type: "board_access_revoked", data: { ...BOARD_REF, reason: "removed" } });
        await settle();
        act(() => {
            vi.advanceTimersByTime(15_000);
        });
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledTimes(1);
    });

    it("never fetches again once the board closes while a failing refetch is in flight", async () => {
        // given
        vi.useFakeTimers();
        catchUp.mockRejectedValue(new Error("forbidden"));
        hold(7);
        const { unmount } = mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        unmount();
        await settle();
        act(() => {
            vi.advanceTimersByTime(15_000);
        });
        await settle();

        // then
        expect(catchUp).toHaveBeenCalledTimes(1);
    });

    it("forgets the refusal of the board it showed before when it moves to another board", () => {
        // given
        holder.ws.reconnect();
        const { result, rerender } = mountOn(BOARD_ID);
        emitRealtimeEvent({ type: "board_join_rejected", data: { ...BOARD_REF, reason: "removed" } });

        // when
        rerender({ boardId: OTHER_BOARD });

        // then
        expect(result.current).toEqual({ state: "connecting" });
    });

    it("never shows the board it showed before as live on the next board", () => {
        // given
        hold(10);
        holder.ws.reconnect();
        const { result, rerender } = mountOn(BOARD_ID);
        emitRealtimeEvent(joined(10));

        // when
        rerender({ boardId: OTHER_BOARD });

        // then
        expect(result.current).toEqual({ state: "connecting" });
    });

    it("ignores a refusal for another board", () => {
        // given
        hold(10);
        const { result } = mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent({ type: "board_join_rejected", data: { board_id: OTHER_BOARD, reason: "removed" } });

        // then
        expect(result.current).toEqual({ state: "live" });
    });
});

describe("useBoardSync and the viewer's own votes", () => {
    function me(version: number, myVotes: Record<string, number>, boardId = BOARD_ID) {
        return {
            type: "board_me" as const,
            data: { board_id: boardId, version, votes_remaining: 4, my_votes: myVotes },
        };
    }

    it("takes the viewer's votes from board_me, ahead of the frame that carries the totals", () => {
        // given
        hold(10);
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent(me(12, { k1: 2 }));

        // then
        expect(held()?.me).toEqual({ votes_remaining: 4, my_votes: { k1: 2 }, version: 12 });
        expect(held()?.version).toBe(10);
    });

    it("keeps the board_me with the highest version", () => {
        // given
        hold(10);
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent(me(13, { k1: 2 }));
        emitRealtimeEvent(me(12, { k1: 1 }));

        // then
        expect(held()?.me.my_votes).toEqual({ k1: 2 });
        expect(held()?.me.version).toBe(13);
    });

    it("never lets an older reset in a later frame wipe out newer votes", () => {
        // given
        hold(10);
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));
        emitRealtimeEvent(me(12, { k1: 1 }));

        // when
        emitRealtimeEvent(frame(11, 12, [{ type: "votes_reset", version: 11 }]));

        // then
        expect(held()?.me.my_votes).toEqual({ k1: 1 });
        expect(held()?.version).toBe(12);
    });

    it("ignores board_me for another board", () => {
        // given
        const before = hold(10);
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent(me(12, { k1: 2 }, OTHER_BOARD));

        // then
        expect(held()).toBe(before);
    });
});

describe("useBoardSync and the team's action items", () => {
    const TEAM = "00000000-0000-4000-8000-0000000000d1";

    function teamChanged(teamId: string) {
        return { type: "team_action_items_changed" as const, data: { team_id: teamId } };
    }

    it("refreshes the previous actions when another board of the board's team changed its items", () => {
        // given
        hold(10, { board: makeBoardView({ team_id: TEAM }) });
        mountOnOpenSocket();
        emitRealtimeEvent(joined(10));

        // when
        emitRealtimeEvent(teamChanged(TEAM));

        // then
        expect(refresh).toHaveBeenCalledWith(queryClient, BOARD_ID);
    });

    it("ignores a change on another team", () => {
        // given
        hold(10, { board: makeBoardView({ team_id: TEAM }) });
        mountOnOpenSocket();

        // when
        emitRealtimeEvent(teamChanged("00000000-0000-4000-8000-0000000000d2"));

        // then
        expect(refresh).not.toHaveBeenCalled();
    });

    it("ignores every team change on an organisation board, which reviews no previous actions", () => {
        // given
        hold(10, { board: makeBoardView({ team_id: undefined }) });
        mountOnOpenSocket();

        // when
        emitRealtimeEvent(teamChanged(TEAM));

        // then
        expect(refresh).not.toHaveBeenCalled();
    });

    it("reports a refresh that failed rather than dropping it", async () => {
        // given
        const failure = new Error("service unavailable");
        refresh.mockRejectedValue(failure);
        hold(10, { board: makeBoardView({ team_id: TEAM }) });
        mountOnOpenSocket();

        // when
        emitRealtimeEvent(teamChanged(TEAM));
        await settle();

        // then
        expect(reportClientError).toHaveBeenCalledWith(failure, { source: "board-refresh" });
    });
});
