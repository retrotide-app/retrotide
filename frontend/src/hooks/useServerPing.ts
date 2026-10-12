import { useEffect, useRef, useState } from "react";
import { REALTIME_EVENTS } from "../api/realtime/events";
import { REALTIME_COMMANDS, sendRealtime } from "../api/realtime/outbound";
import { useRealtimeEvent, useRealtimeStatus } from "../api/realtime/useRealtime";
import { clockOffsetSample, keepLatestSamples, latencyGrade, type LatencyGrade, medianOf } from "../domain/board/clock";

const PROBE_INTERVAL_MS = 1_000;

const PROBE_TIMEOUT_MS = 5_000;

export interface ServerPing {
    roundTripMs: number | null;
    grade: LatencyGrade | null;
    clockOffsetMs: number | null;
}

interface PingReading {
    roundTripMs: number | null;
    clockOffsetMs: number | null;
}

const NO_READING: PingReading = { roundTripMs: null, clockOffsetMs: null };

export function useServerPing(enabled: boolean): ServerPing {
    const epoch = useRealtimeStatus();
    const [reading, setReading] = useState<PingReading>(NO_READING);

    const roundTripsRef = useRef<number[]>([]);
    const offsetsRef = useRef<number[]>([]);
    const pendingRef = useRef(new Map<number, number>());
    const reportedRef = useRef(0);
    const nonceRef = useRef(0);

    useRealtimeEvent(REALTIME_EVENTS.PONG, event => {
        const { nonce, server_now: serverNow } = event.data;
        if (nonce === undefined) {
            return;
        }

        const sentAt = pendingRef.current.get(nonce);
        if (sentAt === undefined) {
            return;
        }

        pendingRef.current.delete(nonce);

        const roundTrip = Math.max(0, performance.now() - sentAt);
        roundTripsRef.current = keepLatestSamples(roundTripsRef.current, roundTrip);

        const offset = serverNow === undefined ? null : clockOffsetSample(serverNow, Date.now(), roundTrip);
        if (offset !== null) {
            offsetsRef.current = keepLatestSamples(offsetsRef.current, offset);
        }

        const median = medianOf(roundTripsRef.current);
        reportedRef.current = median === null ? 0 : Math.round(median);
        setReading({ roundTripMs: median, clockOffsetMs: medianOf(offsetsRef.current) });
    });

    useEffect(() => {
        if (!enabled) {
            return;
        }

        roundTripsRef.current = [];
        offsetsRef.current = [];
        pendingRef.current.clear();
        reportedRef.current = 0;

        const probe = () => {
            const now = performance.now();

            for (const [nonce, sentAt] of pendingRef.current) {
                if (now - sentAt > PROBE_TIMEOUT_MS) {
                    pendingRef.current.delete(nonce);
                }
            }

            nonceRef.current += 1;
            pendingRef.current.set(nonceRef.current, now);
            sendRealtime({ type: REALTIME_COMMANDS.PING, data: { nonce: nonceRef.current, rtt: reportedRef.current } });
        };

        probe();
        const timer = window.setInterval(probe, PROBE_INTERVAL_MS);

        return () => {
            window.clearInterval(timer);
        };
    }, [enabled, epoch]);

    const { roundTripMs, clockOffsetMs } = reading;

    return { roundTripMs, grade: roundTripMs === null ? null : latencyGrade(roundTripMs), clockOffsetMs };
}
