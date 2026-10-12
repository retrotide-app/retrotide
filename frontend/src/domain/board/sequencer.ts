export const RESYNC_EVENT = "resync";

export interface SequencedEvent {
    type: string;
    version: number;
}

export interface SequencedFrame<E extends SequencedEvent> {
    from: number;
    to: number;
    events: readonly E[];
}

export type SequencerPhase = "joining" | "catching-up" | "live" | "refused";

export interface SequencerState<E extends SequencedEvent> {
    phase: SequencerPhase;
    floor: number | null;
    buffered: readonly SequencedFrame<E>[];
}

export interface SequencerApply<E extends SequencedEvent> {
    events: readonly E[];
    to: number;
}

export interface SequencerStep<E extends SequencedEvent> {
    state: SequencerState<E>;
    apply: SequencerApply<E> | null;
    refetch: boolean;
}

interface Drained<E extends SequencedEvent> {
    events: E[];
    cursor: number;
    remaining: SequencedFrame<E>[];
    refetch: boolean;
}

function unchanged<E extends SequencedEvent>(state: SequencerState<E>): SequencerStep<E> {
    return { state, apply: null, refetch: false };
}

function eventsAbove<E extends SequencedEvent>(
    frame: SequencedFrame<E>,
    cursor: number,
): { events: E[]; resyncAt: number | null } {
    const events: E[] = [];

    for (const event of frame.events) {
        if (event.version <= cursor) {
            continue;
        }

        if (event.type === RESYNC_EVENT) {
            return { events, resyncAt: event.version };
        }

        events.push(event);
    }

    return { events, resyncAt: null };
}

function drain<E extends SequencedEvent>(frames: readonly SequencedFrame<E>[], applied: number): Drained<E> {
    const drained: Drained<E> = { events: [], cursor: applied, remaining: [], refetch: false };

    for (const frame of frames.toSorted((left, right) => left.from - right.from)) {
        if (drained.refetch) {
            drained.remaining.push(frame);
            continue;
        }

        if (frame.to <= drained.cursor) {
            continue;
        }

        if (frame.from > drained.cursor + 1) {
            drained.remaining.push(frame);
            drained.refetch = true;
            continue;
        }

        const { events, resyncAt } = eventsAbove(frame, drained.cursor);
        drained.events.push(...events);

        if (resyncAt !== null) {
            drained.cursor = resyncAt - 1;
            drained.remaining.push(frame);
            drained.refetch = true;
            continue;
        }

        drained.cursor = frame.to;
    }

    return drained;
}

export function startSequencer<E extends SequencedEvent>(): SequencerState<E> {
    return { phase: "joining", floor: null, buffered: [] };
}

export function settleSequencer<E extends SequencedEvent>(
    state: SequencerState<E>,
    applied: number | null,
): SequencerStep<E> {
    if (state.floor === null || state.phase === "refused") {
        return unchanged(state);
    }

    if (applied === null || applied < state.floor) {
        return { state: { ...state, phase: "catching-up" }, apply: null, refetch: true };
    }

    const drained = drain(state.buffered, applied);
    const apply = drained.cursor > applied ? { events: drained.events, to: drained.cursor } : null;
    const phase = drained.refetch ? "catching-up" : "live";

    return { state: { phase, floor: state.floor, buffered: drained.remaining }, apply, refetch: drained.refetch };
}

export function receiveJoined<E extends SequencedEvent>(
    state: SequencerState<E>,
    applied: number | null,
    version: number,
): SequencerStep<E> {
    if (state.phase === "refused") {
        return unchanged(state);
    }

    return settleSequencer({ ...state, phase: "catching-up", floor: version }, applied);
}

export function receiveFrame<E extends SequencedEvent>(
    state: SequencerState<E>,
    applied: number | null,
    frame: SequencedFrame<E>,
): SequencerStep<E> {
    if (state.phase === "refused") {
        return unchanged(state);
    }

    const buffered = { ...state, buffered: [...state.buffered, frame] };

    if (state.phase !== "live") {
        return unchanged(buffered);
    }

    return settleSequencer(buffered, applied);
}

export function receiveRefusal<E extends SequencedEvent>(): SequencerState<E> {
    return { phase: "refused", floor: null, buffered: [] };
}
