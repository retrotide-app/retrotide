import { routeTemplate, UNKNOWN_ROUTE_TEMPLATE } from "../domain/routeTemplate";
import type { ClientErrorReportRequest } from "../types/api";
import { apiPost } from "./client";

export type ClientErrorSource =
    | "uncaught"
    | "caught"
    | "recoverable"
    | "window-error"
    | "unhandled-rejection"
    | "boot"
    | "realtime-contract"
    | "board-refresh"
    | "storage"
    | "sound"
    | "turnstile";

export type ClientErrorContext = {
    source: ClientErrorSource;
    componentStack?: string | null;
};

export type ClientErrorReport = ClientErrorReportRequest & {
    source: ClientErrorSource;
};

type ErrorFacts = {
    name: string;
    message: string;
    stack: string | null;
};

type ShrinkableField = "name" | "message" | "stack" | "component_stack";

export const CLIENT_ERROR_PREFIX = "[client-error]";

export const CLIENT_ERROR_SEND_FAILED = "failed to send a client error report";

export const CLIENT_ERRORS_PATH = "/client-errors";

export const MAX_CLIENT_ERROR_REPORTS = 50;

export const MAX_CLIENT_ERROR_REPORT_BYTES = 4096;

const SHRINKABLE_FIELDS: readonly ShrinkableField[] = ["stack", "component_stack", "message", "name"];

const UNKNOWN = "unknown";

const encoder = new TextEncoder();

let sent = 0;

function describeValue(value: unknown): string {
    try {
        if (typeof value === "string") {
            return value;
        }

        return JSON.stringify(value) ?? String(value);
    } catch {
        return "unserialisable thrown value";
    }
}

function describeError(error: unknown): ErrorFacts {
    try {
        if (error instanceof Error) {
            return { name: error.name, message: error.message, stack: error.stack ?? null };
        }

        return { name: "NonError", message: describeValue(error), stack: null };
    } catch {
        return { name: "NonError", message: "unreadable thrown value", stack: null };
    }
}

function currentRoute(): string {
    const pathname = globalThis.location?.pathname;

    if (pathname === undefined) {
        return UNKNOWN_ROUTE_TEMPLATE;
    }

    return routeTemplate(pathname);
}

function appVersion(): string {
    return typeof __APP_VERSION__ === "string" ? __APP_VERSION__ : UNKNOWN;
}

function longestShrinkableField(report: ClientErrorReport): ShrinkableField | null {
    let longest: ShrinkableField | null = null;
    let longestLength = 0;

    for (const field of SHRINKABLE_FIELDS) {
        const length = report[field]?.length ?? 0;

        if (length > longestLength) {
            longest = field;
            longestLength = length;
        }
    }

    return longest;
}

function fitToBodyLimit(report: ClientErrorReport): ClientErrorReport {
    let fitted = report;

    while (encoder.encode(JSON.stringify(fitted)).length > MAX_CLIENT_ERROR_REPORT_BYTES) {
        const field = longestShrinkableField(fitted);

        if (field === null) {
            return fitted;
        }

        const value = fitted[field] ?? "";

        fitted = { ...fitted, [field]: value.slice(0, Math.floor(value.length / 2)) };
    }

    return fitted;
}

export function reportClientError(error: unknown, context: ClientErrorContext): void {
    if (sent >= MAX_CLIENT_ERROR_REPORTS) {
        return;
    }

    sent += 1;

    const facts = describeError(error);

    const report: ClientErrorReport = {
        name: facts.name,
        message: facts.message,
        stack: facts.stack,
        component_stack: context.componentStack ?? null,
        source: context.source,
        route: currentRoute(),
        version: appVersion(),
        sequence: sent,
    };

    apiPost<void, ClientErrorReportRequest>(CLIENT_ERRORS_PATH, fitToBodyLimit(report)).catch((sendError: unknown) => {
        console.error(CLIENT_ERROR_PREFIX, CLIENT_ERROR_SEND_FAILED, sendError);
    });

    console.error(CLIENT_ERROR_PREFIX, report);
}
