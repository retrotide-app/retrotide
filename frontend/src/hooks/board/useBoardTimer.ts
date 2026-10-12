import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { clockOffsetSample } from "../../domain/board/clock";
import { readTimer, type TimerReading } from "../../domain/board/timer";
import { playTimerChime } from "../../platform/sound";
import { getTickerNow, subscribeTicker } from "../../platform/ticker";
import type { BoardSnapshot, TimerView } from "../../types/api";
import { useServerPing } from "../useServerPing";

export interface BoardTimerState {
    timer: TimerView;
    reading: TimerReading;
}

function subscribeNowhere(): () => void {
    return () => {};
}

export function useBoardTimer(snapshot: BoardSnapshot): BoardTimerState | null {
    const { timer, server_now: serverNow, settings } = snapshot;
    const ping = useServerPing(timer !== null);
    const [stampOffsetMs, setStampOffsetMs] = useState<number | null>(null);
    const roundTripRef = useRef(0);
    const armedRef = useRef<string | null>(null);

    const endsAt = timer?.ends_at ?? null;
    const now = useSyncExternalStore(endsAt === null ? subscribeNowhere : subscribeTicker, getTickerNow);

    useEffect(() => {
        roundTripRef.current = ping.roundTripMs ?? 0;
    }, [ping.roundTripMs]);

    useEffect(() => {
        if (serverNow === undefined) {
            return;
        }

        const id = window.setTimeout(() => {
            const sample = clockOffsetSample(serverNow, Date.now(), roundTripRef.current);

            if (sample !== null) {
                setStampOffsetMs(sample);
            }
        }, 0);

        return () => {
            window.clearTimeout(id);
        };
    }, [serverNow]);

    const offsetMs = ping.clockOffsetMs ?? stampOffsetMs ?? 0;
    const reading = timer === null ? null : readTimer(timer, now + offsetMs);
    const finished = reading?.phase === "finished";
    const playSound = settings.timer_play_sound;

    useEffect(() => {
        if (endsAt === null) {
            armedRef.current = null;
            return;
        }

        if (!finished) {
            armedRef.current = endsAt;
            return;
        }

        if (armedRef.current !== endsAt) {
            return;
        }

        armedRef.current = null;

        if (playSound) {
            playTimerChime();
        }
    }, [endsAt, finished, playSound]);

    if (timer === null || reading === null) {
        return null;
    }

    return { timer, reading };
}
