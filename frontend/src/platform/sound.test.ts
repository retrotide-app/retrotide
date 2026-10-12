import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { playTimerChime } from "./sound";

const { reportPlatformError } = vi.hoisted(() => ({ reportPlatformError: vi.fn() }));

vi.mock("./errorReporter", () => ({ reportPlatformError }));

const startOscillator = vi.fn<(at: number) => void>();

const resumeAudio = vi.fn<() => Promise<void>>();

const closeAudio = vi.fn<() => Promise<void>>();

class FakeAudioContext {
    currentTime = 0;
    destination = {};
    resume = resumeAudio;
    close = closeAudio;

    createOscillator() {
        return { type: "", frequency: { value: 0 }, connect: vi.fn(), start: startOscillator, stop: vi.fn() };
    }

    createGain() {
        return { connect: vi.fn(), gain: { setValueAtTime: vi.fn(), exponentialRampToValueAtTime: vi.fn() } };
    }
}

function startTimes(): number[] {
    return startOscillator.mock.calls.map(([at]) => at);
}

beforeEach(() => {
    vi.useFakeTimers();
    startOscillator.mockReset();
    resumeAudio.mockReset();
    resumeAudio.mockResolvedValue(undefined);
    closeAudio.mockReset();
    closeAudio.mockResolvedValue(undefined);
    reportPlatformError.mockReset();
    vi.stubGlobal("AudioContext", FakeAudioContext);
});

afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
});

describe("playTimerChime", () => {
    it("plays a short run of notes one after another, then lets the audio go", async () => {
        // when
        playTimerChime();
        await vi.runAllTimersAsync();

        // then
        const times = startTimes();
        expect(times).toHaveLength(3);
        expect(times[0]).toBeLessThan(times[1]);
        expect(times[1]).toBeLessThan(times[2]);
        expect(closeAudio).toHaveBeenCalledTimes(1);
    });

    it("reports a browser that refuses to start the sound", async () => {
        // given
        const refused = new Error("not allowed");
        resumeAudio.mockRejectedValue(refused);

        // when
        playTimerChime();
        await vi.runAllTimersAsync();

        // then
        expect(reportPlatformError).toHaveBeenCalledWith(refused, "sound");
    });

    it("plays nothing in a browser with no audio", () => {
        // given
        vi.stubGlobal("AudioContext", undefined);

        // when
        playTimerChime();

        // then
        expect(startOscillator).not.toHaveBeenCalled();
        expect(reportPlatformError).not.toHaveBeenCalled();
    });
});
