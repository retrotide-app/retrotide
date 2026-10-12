import { REALTIME_EVENTS, type RealtimeEvent, type RealtimeEventName } from "./events";

export type RealtimeEnvelopeFailure = "invalid-json" | "not-an-object" | "missing-type";

export type RealtimeParseFailure = RealtimeEnvelopeFailure | "unknown-type";

export type RejectedServerEvent =
    | { ok: false; reason: RealtimeEnvelopeFailure }
    | { ok: false; reason: "unknown-type"; type: string };

export type ParseServerEventResult = { ok: true; event: RealtimeEvent } | RejectedServerEvent;

type DecodedJson = { ok: true; value: unknown };

type Envelope = { ok: true; type: RealtimeEventName; data: unknown };

const KNOWN_EVENT_NAMES: ReadonlySet<string> = new Set<string>(Object.values(REALTIME_EVENTS));

function decodeJson(raw: string): DecodedJson | RejectedServerEvent {
    try {
        return { ok: true, value: JSON.parse(raw) as unknown };
    } catch {
        return { ok: false, reason: "invalid-json" };
    }
}

function readEnvelope(value: unknown): Envelope | RejectedServerEvent {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
        return { ok: false, reason: "not-an-object" };
    }

    const { type, data } = value as { type?: unknown; data?: unknown };

    if (typeof type !== "string") {
        return { ok: false, reason: "missing-type" };
    }

    if (!KNOWN_EVENT_NAMES.has(type)) {
        return { ok: false, reason: "unknown-type", type };
    }

    return { ok: true, type: type as RealtimeEventName, data };
}

function asRealtimeEvent(type: RealtimeEventName, data: unknown): RealtimeEvent {
    return { type, data } as RealtimeEvent;
}

export function parseServerEvent(raw: string): ParseServerEventResult {
    const decoded = decodeJson(raw);
    if (!decoded.ok) {
        return decoded;
    }

    const envelope = readEnvelope(decoded.value);
    if (!envelope.ok) {
        return envelope;
    }

    return { ok: true, event: asRealtimeEvent(envelope.type, envelope.data) };
}
