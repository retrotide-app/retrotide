import { isUserSessionHeld, markSessionLost } from "./sessionLost";
import { apiUrl } from "./origin";

const CLIENT_HEADERS: Readonly<Record<string, string>> = { "X-RetroTide-Client": "web" };

const ZERO_MEANS_UNSET_KEYS = new Set(["offset", "page"]);

function isRecord(value: unknown): value is Record<string, unknown> {
    return value !== null && typeof value === "object" && !Array.isArray(value);
}

function readCode(body: unknown): string | null {
    if (!isRecord(body)) {
        return null;
    }

    return typeof body.code === "string" ? body.code : null;
}

export class ApiError extends Error {
    status: number;
    code: string | null;
    body: unknown;

    constructor(status: number, message: string, body: unknown) {
        super(message);
        this.name = "ApiError";
        this.status = status;
        this.code = readCode(body);
        this.body = body;
    }
}

function requestInit(method: string, body?: unknown): RequestInit {
    if (body === undefined) {
        return { method, headers: { ...CLIENT_HEADERS }, credentials: "same-origin" };
    }

    return {
        method,
        headers: { ...CLIENT_HEADERS, "Content-Type": "application/json" },
        body: JSON.stringify(body),
        credentials: "same-origin",
    };
}

async function readErrorBody(response: Response): Promise<unknown> {
    const text = await response.text();

    if (text === "") {
        return null;
    }

    try {
        const decoded: unknown = JSON.parse(text);

        return isRecord(decoded) ? decoded : text;
    } catch {
        return text;
    }
}

async function failIfNotOk(response: Response): Promise<void> {
    if (response.ok) {
        return;
    }

    if (response.status === 401 && isUserSessionHeld()) {
        markSessionLost();
    }

    const body = await readErrorBody(response);
    const message = isRecord(body) && typeof body.error === "string" ? body.error : `API error: ${response.status}`;

    throw new ApiError(response.status, message, body);
}

async function handleResponse<T>(response: Response): Promise<T> {
    await failIfNotOk(response);

    if (response.status === 204 || response.headers.get("content-length") === "0") {
        return undefined as T;
    }

    return (await response.json()) as T;
}

export async function apiFetch<T>(path: string): Promise<T> {
    const response = await fetch(apiUrl(path), requestInit("GET"));

    return handleResponse<T>(response);
}

export async function apiPost<T, B = unknown>(path: string, body?: B): Promise<T> {
    const response = await fetch(apiUrl(path), requestInit("POST", body));

    return handleResponse<T>(response);
}

export async function apiPut<T, B = unknown>(path: string, body: B): Promise<T> {
    const response = await fetch(apiUrl(path), requestInit("PUT", body));

    return handleResponse<T>(response);
}

export async function apiPatch<T, B = unknown>(path: string, body: B): Promise<T> {
    const response = await fetch(apiUrl(path), requestInit("PATCH", body));

    return handleResponse<T>(response);
}

export async function apiDelete<T>(path: string): Promise<T> {
    const response = await fetch(apiUrl(path), requestInit("DELETE"));

    return handleResponse<T>(response);
}

export async function apiUpload<T>(path: string, body: Blob, contentType: string): Promise<T> {
    const response = await fetch(apiUrl(path), {
        method: "POST",
        headers: { ...CLIENT_HEADERS, "Content-Type": contentType },
        body,
        credentials: "same-origin",
    });

    return handleResponse<T>(response);
}

export async function apiDeleteWithBody<T, B = unknown>(path: string, body: B): Promise<T> {
    const response = await fetch(apiUrl(path), requestInit("DELETE", body));

    return handleResponse<T>(response);
}

export function buildQueryString(params: Record<string, string | number | undefined>): string {
    const search = new URLSearchParams();

    for (const [key, value] of Object.entries(params)) {
        if (value === undefined || value === "") {
            continue;
        }

        if (value === 0 && ZERO_MEANS_UNSET_KEYS.has(key)) {
            continue;
        }

        search.set(key, String(value));
    }

    const qs = search.toString();

    return qs ? `?${qs}` : "";
}
