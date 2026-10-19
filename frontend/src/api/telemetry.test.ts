import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { apiPost } = vi.hoisted(() => ({ apiPost: vi.fn() }));

vi.mock("./client", () => ({ apiPost }));

async function loadTelemetryModule(): Promise<typeof import("./telemetry")> {
    vi.resetModules();

    return import("./telemetry");
}

function sentBodies(): unknown[] {
    return apiPost.mock.calls.map(([, body]) => body);
}

beforeEach(() => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    apiPost.mockReset();
    apiPost.mockResolvedValue(undefined);
});

afterEach(() => {
    vi.restoreAllMocks();
    window.history.pushState({}, "", "/");
});

describe("reportClientError", () => {
    it("sends one report to the console sink under a greppable prefix", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();

        // when
        reportClientError(new Error("the board failed to load"), { source: "uncaught" });

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledTimes(1);
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(CLIENT_ERROR_PREFIX, expect.anything());
    });

    it("carries the error, the component stack, the route template and the app version", async () => {
        // given
        window.history.pushState({}, "", "/b/sprint-review");
        vi.stubGlobal("__APP_VERSION__", "7.3.2");
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();
        const error = new TypeError("cards is undefined");

        // when
        reportClientError(error, { source: "caught", componentStack: "\n    at RootErrorBoundary" });

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(CLIENT_ERROR_PREFIX, {
            name: "TypeError",
            message: "cards is undefined",
            stack: error.stack ?? null,
            component_stack: "\n    at RootErrorBoundary",
            source: "caught",
            route: "/b/:slug",
            version: "7.3.2",
            sequence: 1,
        });
    });

    it("sends the same report to the client errors endpoint", async () => {
        // given
        window.history.pushState({}, "", "/o/harbour/boards/00000000-0000-4000-8000-0000000000b1");
        const { CLIENT_ERRORS_PATH, reportClientError } = await loadTelemetryModule();
        const error = new TypeError("cards is undefined");

        // when
        reportClientError(error, { source: "uncaught", componentStack: "\n    at BoardPage" });

        // then
        expect(CLIENT_ERRORS_PATH).toBe("/client-errors");
        expect(apiPost).toHaveBeenCalledExactlyOnceWith(CLIENT_ERRORS_PATH, {
            name: "TypeError",
            message: "cards is undefined",
            stack: error.stack ?? null,
            component_stack: "\n    at BoardPage",
            source: "uncaught",
            route: "/o/:orgSlug/boards/:boardId",
            version: "unknown",
            sequence: 1,
        });
    });

    it("never sends the concrete path, the query or the fragment of the page", async () => {
        // given
        window.history.pushState({}, "", "/b/sprint-review-a1b2c3?code=s3cret#cards");
        const { reportClientError } = await loadTelemetryModule();

        // when
        reportClientError(new Error("the board failed to load"), { source: "uncaught" });

        // then
        const sent = JSON.stringify(sentBodies());
        expect(sent).toContain('"route":"/b/:slug"');
        expect(sent).not.toContain("sprint-review-a1b2c3");
        expect(sent).not.toContain("s3cret");
    });

    it("sends unknown as the route of a page the app has no route for", async () => {
        // given
        window.history.pushState({}, "", "/no-such-page/a1b2c3");
        const { reportClientError } = await loadTelemetryModule();

        // when
        reportClientError(new Error("lost"), { source: "uncaught" });

        // then
        expect(sentBodies()).toEqual([expect.objectContaining({ route: "unknown" })]);
    });

    it("shrinks a report that would not fit the endpoint's body limit, and keeps it whole on the console", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, MAX_CLIENT_ERROR_REPORT_BYTES, reportClientError } = await loadTelemetryModule();
        const error = new Error("€".repeat(2000));
        error.stack = "at frame\n".repeat(2000);

        // when
        reportClientError(error, { source: "caught", componentStack: "\n    at Card".repeat(500) });

        // then
        const [body] = sentBodies();
        expect(new TextEncoder().encode(JSON.stringify(body)).length).toBeLessThanOrEqual(
            MAX_CLIENT_ERROR_REPORT_BYTES,
        );
        expect(body).toEqual(expect.objectContaining({ name: "Error", source: "caught", sequence: 1 }));
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ message: error.message, stack: error.stack }),
        );
    });

    it("sends a report that fits the body limit untouched", async () => {
        // given
        const { reportClientError } = await loadTelemetryModule();
        const error = new Error("a".repeat(1000));
        error.stack = "Error: aaa\n    at Board (index-a1b2c3.js:1:2)";

        // when
        reportClientError(error, { source: "caught" });

        // then
        expect(sentBodies()).toEqual([expect.objectContaining({ message: error.message, stack: error.stack })]);
    });

    it("writes a failed send to the console once and never reports it as a client error", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, CLIENT_ERROR_SEND_FAILED, reportClientError } = await loadTelemetryModule();
        const failure = new Error("too many requests, please try again later");
        apiPost.mockRejectedValue(failure);

        // when
        reportClientError(new Error("boom"), { source: "uncaught" });

        // then
        await vi.waitFor(() => {
            expect(vi.mocked(console.error)).toHaveBeenCalledWith(
                CLIENT_ERROR_PREFIX,
                CLIENT_ERROR_SEND_FAILED,
                failure,
            );
        });
        expect(apiPost).toHaveBeenCalledTimes(1);
        expect(vi.mocked(console.error)).toHaveBeenCalledTimes(2);
    });

    it("stops sending to the endpoint once the session cap is reached", async () => {
        // given
        const { MAX_CLIENT_ERROR_REPORTS, reportClientError } = await loadTelemetryModule();

        // when
        for (let i = 0; i < MAX_CLIENT_ERROR_REPORTS + 5; i += 1) {
            reportClientError(new Error(`render loop ${i}`), { source: "recoverable" });
        }

        // then
        expect(apiPost).toHaveBeenCalledTimes(MAX_CLIENT_ERROR_REPORTS);
    });

    it("says the version is unknown when the build stamped none", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();

        // when
        reportClientError(new Error("no stamp"), { source: "boot" });

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ version: "unknown" }),
        );
    });

    it("records a null component stack when the caller has none", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();

        // when
        reportClientError(new Error("no stack"), { source: "window-error" });

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ component_stack: null }),
        );
    });

    it("sends every report up to the session cap", async () => {
        // given
        const { MAX_CLIENT_ERROR_REPORTS, reportClientError } = await loadTelemetryModule();

        // when
        for (let i = 0; i < MAX_CLIENT_ERROR_REPORTS; i += 1) {
            reportClientError(new Error(`render loop ${i}`), { source: "recoverable" });
        }

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledTimes(MAX_CLIENT_ERROR_REPORTS);
    });

    it("stops sending once the session cap is reached", async () => {
        // given
        const { MAX_CLIENT_ERROR_REPORTS, reportClientError } = await loadTelemetryModule();

        // when
        for (let i = 0; i < MAX_CLIENT_ERROR_REPORTS * 4; i += 1) {
            reportClientError(new Error(`render loop ${i}`), { source: "recoverable" });
        }

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledTimes(MAX_CLIENT_ERROR_REPORTS);
    });

    it("stays silent on every further call past the cap", async () => {
        // given
        const { MAX_CLIENT_ERROR_REPORTS, reportClientError } = await loadTelemetryModule();
        for (let i = 0; i < MAX_CLIENT_ERROR_REPORTS; i += 1) {
            reportClientError(new Error(`render loop ${i}`), { source: "recoverable" });
        }
        vi.mocked(console.error).mockClear();

        // when
        reportClientError(new Error("first past the cap"), { source: "recoverable" });
        reportClientError(new Error("second past the cap"), { source: "recoverable" });

        // then
        expect(vi.mocked(console.error)).not.toHaveBeenCalled();
    });

    it("numbers each report so a capped session shows where it stopped", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();

        // when
        reportClientError(new Error("first"), { source: "caught" });
        reportClientError(new Error("second"), { source: "caught" });

        // then
        expect(vi.mocked(console.error)).toHaveBeenLastCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ sequence: 2 }),
        );
    });

    it("still reports a thrown value that is not an Error", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();

        // when
        reportClientError("a bare string was thrown", { source: "unhandled-rejection" });

        // then
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ name: "NonError", message: "a bare string was thrown", stack: null }),
        );
    });

    it("still reports a thrown value that cannot be serialised", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();
        const circular: Record<string, unknown> = {};
        circular.self = circular;

        // when
        const report = () => reportClientError(circular, { source: "unhandled-rejection" });

        // then
        expect(report).not.toThrow();
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ name: "NonError", message: "unserialisable thrown value" }),
        );
    });

    it("still reports an Error whose own fields throw when read", async () => {
        // given
        const { CLIENT_ERROR_PREFIX, reportClientError } = await loadTelemetryModule();
        const hostile = new Error("readable");
        Object.defineProperty(hostile, "message", {
            get() {
                throw new Error("this field is a trap");
            },
        });

        // when
        const report = () => reportClientError(hostile, { source: "caught" });

        // then
        expect(report).not.toThrow();
        expect(vi.mocked(console.error)).toHaveBeenCalledWith(
            CLIENT_ERROR_PREFIX,
            expect.objectContaining({ name: "NonError", message: "unreadable thrown value" }),
        );
    });

    it("lets a failure of the console sink itself reach the caller", async () => {
        // given
        const { reportClientError } = await loadTelemetryModule();
        vi.mocked(console.error).mockImplementation(() => {
            throw new Error("the console is gone");
        });

        // when
        const report = () => reportClientError(new Error("boom"), { source: "uncaught" });

        // then
        expect(report).toThrow("the console is gone");
    });

    it("refuses a caller that hands it no context at all", async () => {
        // given
        const { reportClientError } = await loadTelemetryModule();
        const untyped = reportClientError as (error: unknown, context: unknown) => void;

        // when
        const report = () => untyped(new Error("boom"), null);

        // then
        expect(report).toThrow(TypeError);
    });
});
