export type PlatformErrorSource = "storage" | "sound";

export type PlatformErrorReporter = (error: unknown, context: { source: PlatformErrorSource }) => void;

const UNREGISTERED_PREFIX = "[platform-error]";

let reporter: PlatformErrorReporter | null = null;

export function setPlatformErrorReporter(report: PlatformErrorReporter): void {
    reporter = report;
}

export function reportPlatformError(error: unknown, source: PlatformErrorSource): void {
    if (reporter) {
        reporter(error, { source });
        return;
    }

    console.error(UNREGISTERED_PREFIX, source, error);
}
