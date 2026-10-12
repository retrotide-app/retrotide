import { dispatch } from "./bus";
import { passesRealtimeGuard } from "./guards";
import { parseServerEvent, type RejectedServerEvent } from "./parse";
import { onFrame, onStatus } from "./socket";
import { reportClientError } from "../telemetry";

type EpochListener = () => void;

const epochListeners = new Set<EpochListener>();

let epoch = 0;
let connected = false;
let installed = false;

const reportedParseFailures = new Set<string>();
const reportedUnknownTypes = new Set<string>();
const reportedGuardFailures = new Set<string>();

function reportOnce(reported: Set<string>, key: string, message: string): void {
    if (reported.has(key)) {
        return;
    }

    reported.add(key);
    reportClientError(new Error(message), { source: "realtime-contract" });
}

function reportRejectedFrame(rejected: RejectedServerEvent): void {
    if (rejected.reason === "unknown-type") {
        reportOnce(reportedUnknownTypes, rejected.type, `unknown realtime event "${rejected.type}"`);
        return;
    }

    reportOnce(reportedParseFailures, rejected.reason, `realtime frame rejected: ${rejected.reason}`);
}

function receiveFrame(raw: string): void {
    const parsed = parseServerEvent(raw);
    if (!parsed.ok) {
        reportRejectedFrame(parsed);
        return;
    }

    const { event } = parsed;

    if (!passesRealtimeGuard(event)) {
        reportOnce(reportedGuardFailures, event.type, `realtime event "${event.type}" failed its payload guard`);
        return;
    }

    dispatch(event);
}

function notifyEpochListeners(): void {
    for (const listener of Array.from(epochListeners)) {
        listener();
    }
}

function receiveStatus(open: boolean): void {
    if (open) {
        epoch += 1;
        connected = true;
        notifyEpochListeners();
        return;
    }

    if (!connected) {
        return;
    }

    connected = false;
    notifyEpochListeners();
}

export function ensureRealtimePipeline(): void {
    if (installed) {
        return;
    }

    installed = true;

    onFrame(receiveFrame);
    onStatus(receiveStatus);
}

export function getRealtimeEpoch(): number {
    return epoch;
}

export function getRealtimeConnected(): boolean {
    return connected;
}

export function subscribeRealtimeEpoch(listener: EpochListener): () => void {
    ensureRealtimePipeline();

    epochListeners.add(listener);

    return () => {
        epochListeners.delete(listener);
    };
}
