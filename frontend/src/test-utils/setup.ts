import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

class MockObserver {
    observe = vi.fn();
    unobserve = vi.fn();
    disconnect = vi.fn();
    takeRecords = vi.fn(() => []);
}

function define(target: object, members: Record<string, unknown>): void {
    for (const [key, value] of Object.entries(members)) {
        Object.defineProperty(target, key, { configurable: true, writable: true, value });
    }
}

if (!window.matchMedia) {
    define(window, {
        matchMedia: vi.fn((query: string) => ({
            matches: false,
            media: query,
            onchange: null,
            addListener: vi.fn(),
            removeListener: vi.fn(),
            addEventListener: vi.fn(),
            removeEventListener: vi.fn(),
            dispatchEvent: vi.fn(() => false),
        })),
    });
}

define(globalThis, { ResizeObserver: MockObserver, IntersectionObserver: MockObserver });
define(Element.prototype, { scrollIntoView: vi.fn(), scrollTo: vi.fn() });
define(window, { scrollTo: vi.fn() });

afterEach(() => {
    cleanup();
    localStorage.clear();
    sessionStorage.clear();
    vi.useRealTimers();
});
