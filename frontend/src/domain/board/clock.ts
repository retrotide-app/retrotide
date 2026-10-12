export const LATENCY_FAIR_MS = 90;

export const LATENCY_POOR_MS = 170;

export const LATENCY_SAMPLES = 5;

export type LatencyGrade = "good" | "fair" | "poor";

export function latencyGrade(roundTripMs: number): LatencyGrade {
    if (roundTripMs >= LATENCY_POOR_MS) {
        return "poor";
    }

    if (roundTripMs >= LATENCY_FAIR_MS) {
        return "fair";
    }

    return "good";
}

export function medianOf(samples: readonly number[]): number | null {
    if (samples.length === 0) {
        return null;
    }

    const sorted = samples.toSorted((left, right) => left - right);
    const middle = Math.floor(sorted.length / 2);

    if (sorted.length % 2 === 1) {
        return sorted[middle];
    }

    return (sorted[middle - 1] + sorted[middle]) / 2;
}

export function keepLatestSamples(samples: readonly number[], next: number): number[] {
    return [...samples, next].slice(-LATENCY_SAMPLES);
}

export function clockOffsetSample(serverNow: string, receivedAtMs: number, roundTripMs: number): number | null {
    const serverMs = Date.parse(serverNow);

    if (Number.isNaN(serverMs)) {
        return null;
    }

    return serverMs + roundTripMs / 2 - receivedAtMs;
}
