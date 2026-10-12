import mainSource from "./main.tsx?raw";
import { act } from "@testing-library/react";
import type { ReactElement } from "react";
import type { RootOptions } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { createRoot, renderRoot, reportClientError, setPlatformErrorReporter } = vi.hoisted(() => ({
    createRoot: vi.fn(),
    renderRoot: vi.fn(),
    reportClientError: vi.fn(),
    setPlatformErrorReporter: vi.fn(),
}));

vi.mock("react-dom/client", () => ({ createRoot }));
vi.mock("./api/telemetry", () => ({ reportClientError }));
vi.mock("./platform/errorReporter", () => ({ setPlatformErrorReporter }));
vi.mock("./App", () => ({
    default: function App() {
        return null;
    },
}));

type AnyElement = ReactElement<Record<string, unknown>>;

type ErrorCallbacks = Required<Pick<RootOptions, "onUncaughtError" | "onCaughtError" | "onRecoverableError">>;

const registered: Array<[string, EventListenerOrEventListenerObject]> = [];

const providerFailure = new Error("a provider gave up");

function CollapsingProvider(): never {
    throw providerFailure;
}

function isElementLike(node: unknown): node is AnyElement {
    return typeof node === "object" && node !== null && "type" in node && "props" in node;
}

function elementOr(node: unknown, missing: string): AnyElement {
    if (!isElementLike(node)) {
        throw new Error(missing);
    }

    return node;
}

function providerChain(node: unknown): string[] {
    const names: string[] = [];
    let current: unknown = node;

    while (isElementLike(current)) {
        names.push(typeof current.type === "function" ? current.type.name : String(current.type));
        current = current.props.children;
    }

    return names;
}

async function bootMain(): Promise<void> {
    vi.resetModules();

    await import("./main");
}

function renderedTree(): AnyElement {
    return elementOr(renderRoot.mock.calls[0][0], "nothing was rendered");
}

function mountedBoundary(): AnyElement {
    return elementOr(renderedTree().props.children, "the root error boundary was not mounted");
}

function errorCallbacks(): ErrorCallbacks {
    const options: RootOptions = createRoot.mock.calls[0][1];

    if (!options.onUncaughtError || !options.onCaughtError || !options.onRecoverableError) {
        throw new Error("createRoot was handed no error callbacks");
    }

    return {
        onUncaughtError: options.onUncaughtError,
        onCaughtError: options.onCaughtError,
        onRecoverableError: options.onRecoverableError,
    };
}

function dispatchRejection(reason: unknown): void {
    const event = new Event("unhandledrejection");
    Object.defineProperty(event, "reason", { value: reason });

    window.dispatchEvent(event);
}

beforeEach(() => {
    document.body.innerHTML = '<div id="root"></div>';

    const original = window.addEventListener.bind(window);
    vi.spyOn(window, "addEventListener").mockImplementation((type, handler, options) => {
        if (handler) {
            registered.push([type, handler]);
        }

        original(type, handler, options);
    });

    createRoot.mockReturnValue({ render: renderRoot, unmount: vi.fn() });
});

afterEach(() => {
    for (const [type, handler] of registered) {
        window.removeEventListener(type, handler);
    }

    registered.length = 0;
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

describe("main", () => {
    it("installs the global error handlers before any other module is evaluated", () => {
        // given
        const lines = mainSource.split("\n");

        // when
        const firstImport = lines.find((line: string) => line.startsWith("import"));

        // then
        expect(firstImport).toBe('import "./api/globalErrorHandlers";');
    });

    it("points the platform layer at the same reporter as everything else", async () => {
        // given
        const reporter = reportClientError;

        // when
        await bootMain();

        // then
        expect(setPlatformErrorReporter).toHaveBeenCalledWith(reporter);
    });

    it("renders the app once, into the root element", async () => {
        // given
        const root = document.getElementById("root");

        // when
        await bootMain();

        // then
        expect(createRoot).toHaveBeenCalledTimes(1);
        expect(createRoot.mock.calls[0][0]).toBe(root);
        expect(renderRoot).toHaveBeenCalledTimes(1);
    });

    it("reports a render error that no boundary caught", async () => {
        // given
        await bootMain();
        const error = new Error("the board would not draw");

        // when
        errorCallbacks().onUncaughtError(error, { componentStack: "\n    at Column" });

        // then
        expect(reportClientError).toHaveBeenCalledWith(error, {
            source: "uncaught",
            componentStack: "\n    at Column",
        });
    });

    it("reports a render error a boundary caught, which is how a lazy child is covered", async () => {
        // given
        await bootMain();
        const error = new Error("the board would not draw");

        // when
        errorCallbacks().onCaughtError(error, { componentStack: "\n    at Column" });

        // then
        expect(reportClientError).toHaveBeenCalledWith(error, {
            source: "caught",
            componentStack: "\n    at Column",
        });
    });

    it("reports an error React recovered from on its own", async () => {
        // given
        await bootMain();
        const error = new Error("the render was retried");

        // when
        errorCallbacks().onRecoverableError(error, { componentStack: "\n    at Column" });

        // then
        expect(reportClientError).toHaveBeenCalledWith(error, {
            source: "recoverable",
            componentStack: "\n    at Column",
        });
    });

    it("reports an error that escaped everything and reached the window", async () => {
        // given
        await bootMain();
        const error = new Error("the card fell off the board");

        // when
        window.dispatchEvent(new ErrorEvent("error", { error, message: "the card fell off the board" }));

        // then
        expect(reportClientError).toHaveBeenCalledWith(error, { source: "window-error" });
    });

    it("falls back to the message when the browser hands over no error object", async () => {
        // given
        await bootMain();

        // when
        window.dispatchEvent(new ErrorEvent("error", { error: null, message: "Script error." }));

        // then
        expect(reportClientError).toHaveBeenCalledWith("Script error.", { source: "window-error" });
    });

    it("reports a promise rejection nobody caught", async () => {
        // given
        await bootMain();
        const reason = new Error("the fetch never landed");

        // when
        dispatchRejection(reason);

        // then
        expect(reportClientError).toHaveBeenCalledWith(reason, { source: "unhandled-rejection" });
    });

    it("puts the boundary above every provider so a provider throw still shows a fallback", async () => {
        // given
        await bootMain();
        const [{ RootErrorBoundary }, { StrictMode }] = await Promise.all([
            import("./components/RootErrorBoundary/RootErrorBoundary"),
            import("react"),
        ]);

        // when
        const tree = renderedTree();
        const boundary = mountedBoundary();

        // then
        expect(tree.type).toBe(StrictMode);
        expect(boundary.type).toBe(RootErrorBoundary);
        expect(providerChain(boundary.props.children)).toEqual([
            "QueryClientProvider",
            "SiteInfoProvider",
            "AuthProvider",
            "OrgProvider",
            "ThemeProvider",
            "App",
        ]);
    });

    it("reports a crash the root boundary caught exactly once, through the root's own callback", async () => {
        // given
        await bootMain();
        const [{ createRoot: createRealRoot }, { RootErrorBoundary }] = await Promise.all([
            vi.importActual<typeof import("react-dom/client")>("react-dom/client"),
            import("./components/RootErrorBoundary/RootErrorBoundary"),
        ]);
        const options: RootOptions = createRoot.mock.calls[0][1];
        const root = createRealRoot(document.createElement("div"), options);

        // when
        act(() => {
            root.render(
                <RootErrorBoundary>
                    <CollapsingProvider />
                </RootErrorBoundary>,
            );
        });

        // then
        expect(reportClientError).toHaveBeenCalledExactlyOnceWith(providerFailure, {
            source: "caught",
            componentStack: expect.stringContaining("CollapsingProvider"),
        });
    });

    it("refuses to boot, and renders nothing, when the page has no root element", async () => {
        // given
        document.body.innerHTML = "";
        vi.resetModules();

        // when
        const booting = import("./main");

        // then
        await expect(booting).rejects.toThrow("index.html has no #root element");
        expect(createRoot).not.toHaveBeenCalled();
        expect(renderRoot).not.toHaveBeenCalled();
    });
});
