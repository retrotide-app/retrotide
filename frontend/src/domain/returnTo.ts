export interface ReturnState {
    from: string;
}

const API_ROOT = "/api";

function hasControlCharacter(path: string): boolean {
    for (const character of path) {
        const code = character.charCodeAt(0);

        if (code < 0x20 || code === 0x7f) {
            return true;
        }
    }

    return false;
}

export function isSafeReturnPath(path: string): boolean {
    if (!path.startsWith("/") || path.startsWith("//")) {
        return false;
    }

    if (path.includes("\\") || hasControlCharacter(path)) {
        return false;
    }

    const lowered = path.toLowerCase();

    return lowered !== API_ROOT && !lowered.startsWith(`${API_ROOT}/`) && !lowered.startsWith(`${API_ROOT}?`);
}

export function returnState(pathname: string, search: string): ReturnState {
    return { from: `${pathname}${search}` };
}

export function returnPathFrom(state: unknown): string | null {
    if (state === null || typeof state !== "object") {
        return null;
    }

    const from = (state as Partial<Record<keyof ReturnState, unknown>>).from;

    if (typeof from !== "string" || !isSafeReturnPath(from)) {
        return null;
    }

    return from;
}
