import { reportPlatformError } from "./errorReporter";

const CHIME_NOTES_HZ: readonly number[] = [880, 660, 880];

const NOTE_GAP_S = 0.22;

const NOTE_LENGTH_S = 0.4;

const PEAK_GAIN = 0.2;

const QUIET_GAIN = 0.0001;

const CLOSE_AFTER_MS = 1_500;

export function playTimerChime(): void {
    if (typeof window.AudioContext !== "function") {
        return;
    }

    const context = new window.AudioContext();
    const start = context.currentTime;

    for (const [index, hz] of CHIME_NOTES_HZ.entries()) {
        const at = start + index * NOTE_GAP_S;
        const oscillator = context.createOscillator();
        const gain = context.createGain();

        oscillator.type = "sine";
        oscillator.frequency.value = hz;
        gain.gain.setValueAtTime(QUIET_GAIN, at);
        gain.gain.exponentialRampToValueAtTime(PEAK_GAIN, at + 0.02);
        gain.gain.exponentialRampToValueAtTime(QUIET_GAIN, at + NOTE_LENGTH_S);

        oscillator.connect(gain);
        gain.connect(context.destination);
        oscillator.start(at);
        oscillator.stop(at + NOTE_LENGTH_S);
    }

    context.resume().catch((error: unknown) => {
        reportPlatformError(error, "sound");
    });

    window.setTimeout(() => {
        context.close().catch((error: unknown) => {
            reportPlatformError(error, "sound");
        });
    }, CLOSE_AFTER_MS);
}
