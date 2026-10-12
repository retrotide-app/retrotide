export const API_PREFIX = "/api/v1";

export function apiUrl(path: string): string {
    return `${API_PREFIX}${path}`;
}

export function socketUrl(origin: string): string {
    const socketOrigin = origin.replace(/^http(s?):\/\//, "ws$1://");

    return `${socketOrigin}${API_PREFIX}/ws`;
}
