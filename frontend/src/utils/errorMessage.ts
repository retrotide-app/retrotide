function readField(thrown: unknown, field: string): unknown {
    if (thrown === null || typeof thrown !== "object") {
        return undefined;
    }

    return (thrown as Record<string, unknown>)[field];
}

function readMessage(thrown: unknown): string {
    if (typeof thrown === "string") {
        return thrown;
    }

    const message = readField(thrown, "message");

    return typeof message === "string" ? message : "";
}

export function errorMessage(e: unknown, fallback: string): string {
    const message = readMessage(e).trim();

    return message === "" ? fallback : message;
}

export function errorCode(e: unknown): string | null {
    const code = readField(e, "code");

    return typeof code === "string" && code !== "" ? code : null;
}

export function errorStatus(e: unknown): number | null {
    const status = readField(e, "status");

    return typeof status === "number" ? status : null;
}
