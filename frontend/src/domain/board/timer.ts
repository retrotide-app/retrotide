import type { TimerView } from "../../types/api";

export type TimerPhase = "running" | "paused" | "finished";

export type TimerTone = "calm" | "amber" | "red";

export interface TimerReading {
    remainingMs: number;
    phase: TimerPhase;
    tone: TimerTone;
}

export const MINUTE_MS = 60_000;

export const TIMER_MAX_MS = 60 * MINUTE_MS;

export const TIMER_AMBER_MS = MINUTE_MS;

export const TIMER_RED_MS = 15_000;

export const TIMER_LENGTHS_MINUTES: readonly number[] = [1, 2, 3, 5, 10, 15, 20, 30, 45, 60];

export const DEFAULT_TIMER_MINUTES = 5;

const SECOND_MS = 1_000;

export function timerRemainingMs(timer: TimerView, serverNowMs: number): number {
    if (timer.paused_remaining_ms !== null || timer.ends_at === null) {
        return Math.max(0, timer.paused_remaining_ms ?? 0);
    }

    return Math.max(0, Date.parse(timer.ends_at) - serverNowMs);
}

export function timerTone(remainingMs: number, durationMs: number): TimerTone {
    if (remainingMs <= Math.min(TIMER_RED_MS, durationMs / 10)) {
        return "red";
    }

    if (remainingMs <= Math.min(TIMER_AMBER_MS, durationMs / 4)) {
        return "amber";
    }

    return "calm";
}

export function readTimer(timer: TimerView, serverNowMs: number): TimerReading {
    const remainingMs = timerRemainingMs(timer, serverNowMs);
    const paused = timer.paused_remaining_ms !== null;

    let phase: TimerPhase = "running";

    if (paused) {
        phase = "paused";
    } else if (remainingMs === 0) {
        phase = "finished";
    }

    return { remainingMs, phase, tone: timerTone(remainingMs, timer.duration_ms) };
}

export function formatTimer(remainingMs: number): string {
    const seconds = Math.ceil(Math.max(0, remainingMs) / SECOND_MS);
    const minutes = Math.floor(seconds / 60);
    const rest = seconds % 60;

    return `${minutes}:${String(rest).padStart(2, "0")}`;
}

export function canAddMinute(timer: TimerView, reading: TimerReading): boolean {
    return reading.phase === "finished" || timer.duration_ms + MINUTE_MS <= TIMER_MAX_MS;
}

export function timerLengthMs(minutes: number): number {
    return minutes * MINUTE_MS;
}

export function timerLengthLabel(minutes: number): string {
    return minutes === 1 ? "1 minute" : `${minutes} minutes`;
}
