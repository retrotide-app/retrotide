const ALLOWED_PROTOCOLS: ReadonlySet<string> = new Set(["https:", "http:"]);

export function readLocationToken(): string {
    const fragment = window.location.hash.replace(/^#/, "");

    try {
        return decodeURIComponent(fragment);
    } catch {
        return fragment;
    }
}

export function clearLocationToken(): void {
    if (window.location.hash === "") {
        return;
    }

    window.history.replaceState(window.history.state, "", `${window.location.pathname}${window.location.search}`);
}

export function pageOrigin(): string {
    return window.location.origin;
}

export function leaveForExternalPage(url: string): void {
    const destination = new URL(url, window.location.href);

    if (!ALLOWED_PROTOCOLS.has(destination.protocol)) {
        throw new Error(`refusing to leave for a ${destination.protocol} address`);
    }

    window.location.assign(destination.href);
}
