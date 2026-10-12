export const TICK_MS = 250;

const listeners = new Set<() => void>();

let now = Date.now();

let interval: ReturnType<typeof setInterval> | null = null;

function tick(): void {
    now = Date.now();

    for (const listener of listeners) {
        listener();
    }
}

export function subscribeTicker(listener: () => void): () => void {
    listeners.add(listener);
    now = Date.now();

    if (interval === null) {
        interval = setInterval(tick, TICK_MS);
    }

    return () => {
        listeners.delete(listener);

        if (listeners.size === 0 && interval !== null) {
            clearInterval(interval);
            interval = null;
        }
    };
}

export function getTickerNow(): number {
    return now;
}
