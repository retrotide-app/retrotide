import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

async function loadReporterModule(): Promise<typeof import("./errorReporter")> {
    vi.resetModules();

    return import("./errorReporter");
}

beforeEach(() => {
    vi.resetModules();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe("reportPlatformError", () => {
    it("hands the failure to the reporter the composition root registered", async () => {
        // given
        const { reportPlatformError, setPlatformErrorReporter } = await loadReporterModule();
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new Error("the quota is exhausted");

        // when
        reportPlatformError(failure, "storage");

        // then
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });

    it("keeps the console quiet once a reporter has been registered", async () => {
        // given
        const { reportPlatformError, setPlatformErrorReporter } = await loadReporterModule();
        const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
        setPlatformErrorReporter(vi.fn());

        // when
        reportPlatformError(new Error("the quota is exhausted"), "storage");

        // then
        expect(consoleError).not.toHaveBeenCalled();
    });

    it("hands the failure only to the reporter registered last", async () => {
        // given
        const { reportPlatformError, setPlatformErrorReporter } = await loadReporterModule();
        const first = vi.fn();
        const second = vi.fn();
        setPlatformErrorReporter(first);
        setPlatformErrorReporter(second);
        const failure = new Error("the quota is exhausted");

        // when
        reportPlatformError(failure, "storage");

        // then
        expect(first).not.toHaveBeenCalled();
        expect(second).toHaveBeenCalledWith(failure, { source: "storage" });
    });

    it("writes the failure and its source to the console when nothing has been registered yet", async () => {
        // given
        const { reportPlatformError } = await loadReporterModule();
        const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
        const failure = new Error("too early");

        // when
        reportPlatformError(failure, "storage");

        // then
        expect(consoleError).toHaveBeenCalledWith("[platform-error]", "storage", failure);
    });

    it("does not throw when nothing has been registered yet", async () => {
        // given
        const { reportPlatformError } = await loadReporterModule();
        vi.spyOn(console, "error").mockImplementation(() => {});

        // when
        const attempt = () => reportPlatformError(new Error("too early"), "storage");

        // then
        expect(attempt).not.toThrow();
    });
});
