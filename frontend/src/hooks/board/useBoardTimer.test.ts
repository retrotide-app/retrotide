import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { makeBoardSettings, makeBoardSnapshot, makeTimer } from "../../test-utils/fixtures";
import type { BoardSnapshot } from "../../types/api";
import type { ServerPing } from "../useServerPing";
import { useBoardTimer } from "./useBoardTimer";

const { useServerPing } = vi.hoisted(() => ({ useServerPing: vi.fn<(enabled: boolean) => ServerPing>() }));

const { playTimerChime } = vi.hoisted(() => ({ playTimerChime: vi.fn() }));

vi.mock("../useServerPing", () => ({ useServerPing }));
vi.mock("../../platform/sound", () => ({ playTimerChime }));

const LOCAL_NOW = Date.parse("2026-10-01T09:30:00.000Z");

const NO_PING: ServerPing = { roundTripMs: null, grade: null, clockOffsetMs: null };

function running(endsInMs: number, serverAheadMs = 0): BoardSnapshot {
    const serverNow = LOCAL_NOW + serverAheadMs;
    const endsAt = new Date(serverNow + endsInMs).toISOString();

    return makeBoardSnapshot({
        timer: makeTimer({ duration_ms: 300_000, ends_at: endsAt, paused_remaining_ms: null }),
        server_now: new Date(serverNow).toISOString(),
    });
}

function renderTimer(snapshot: BoardSnapshot) {
    return renderHook((props: { snapshot: BoardSnapshot }) => useBoardTimer(props.snapshot), {
        initialProps: { snapshot },
    });
}

beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(LOCAL_NOW);
    useServerPing.mockReset();
    useServerPing.mockReturnValue(NO_PING);
    playTimerChime.mockReset();
});

afterEach(() => {
    vi.useRealTimers();
});

describe("useBoardTimer", () => {
    it("shows nothing and sends no pings while no timer is set", () => {
        // when
        const { result } = renderTimer(makeBoardSnapshot({ timer: null }));

        // then
        expect(result.current).toBeNull();
        expect(useServerPing).toHaveBeenLastCalledWith(false);
    });

    it("pings the server while a timer is set, for the clock offset", () => {
        // when
        renderTimer(running(120_000));

        // then
        expect(useServerPing).toHaveBeenLastCalledWith(true);
    });

    it("counts down on the server's clock, using the offset the pings measured", () => {
        // given
        useServerPing.mockReturnValue({ roundTripMs: 40, grade: "good", clockOffsetMs: 10_000 });

        // when
        const { result } = renderTimer(running(120_000, 10_000));

        // then
        expect(result.current?.reading).toEqual({ remainingMs: 120_000, phase: "running", tone: "calm" });
    });

    it("falls back on the server's clock in the snapshot until a ping answers", () => {
        // given
        const { result } = renderTimer(running(120_000, 30_000));

        // when
        act(() => {
            vi.advanceTimersByTime(0);
        });

        // then
        expect(result.current?.reading.remainingMs).toBe(120_000);
    });

    it("ticks down as time passes", () => {
        // given
        const { result } = renderTimer(running(120_000));

        // when
        act(() => {
            vi.advanceTimersByTime(1_000);
        });

        // then
        expect(result.current?.reading.remainingMs).toBe(119_000);
    });

    it("holds a paused timer still", () => {
        // given
        const { result } = renderTimer(
            makeBoardSnapshot({ timer: makeTimer({ ends_at: null, paused_remaining_ms: 42_000 }) }),
        );

        // when
        act(() => {
            vi.advanceTimersByTime(5_000);
        });

        // then
        expect(result.current?.reading).toEqual({ remainingMs: 42_000, phase: "paused", tone: "amber" });
    });

    it("turns amber, then red, as the end nears", () => {
        // given
        const { result } = renderTimer(running(61_000));

        // when
        act(() => {
            vi.advanceTimersByTime(1_000);
        });
        const amber = result.current?.reading.tone;
        act(() => {
            vi.advanceTimersByTime(45_000);
        });

        // then
        expect(amber).toBe("amber");
        expect(result.current?.reading.tone).toBe("red");
    });

    it("chimes once when a running timer reaches its end, while the sound is on", () => {
        // given
        const { result } = renderTimer(running(2_000));

        // when
        act(() => {
            vi.advanceTimersByTime(5_000);
        });

        // then
        expect(result.current?.reading.phase).toBe("finished");
        expect(playTimerChime).toHaveBeenCalledTimes(1);
    });

    it("stays quiet at the end while the sound is off", () => {
        // given
        const snapshot = running(2_000);
        renderTimer({ ...snapshot, settings: makeBoardSettings({ timer_play_sound: false }) });

        // when
        act(() => {
            vi.advanceTimersByTime(5_000);
        });

        // then
        expect(playTimerChime).not.toHaveBeenCalled();
    });

    it("stays quiet when the board opens on a timer that has already run out", () => {
        // when
        renderTimer(running(-5_000));
        act(() => {
            vi.advanceTimersByTime(1_000);
        });

        // then
        expect(playTimerChime).not.toHaveBeenCalled();
    });

    it("chimes again for a minute added after the end", () => {
        // given
        const { rerender } = renderTimer(running(1_000));
        act(() => {
            vi.advanceTimersByTime(2_000);
        });

        // when
        rerender({ snapshot: running(60_000) });
        act(() => {
            vi.advanceTimersByTime(61_000);
        });

        // then
        expect(playTimerChime).toHaveBeenCalledTimes(2);
    });
});
