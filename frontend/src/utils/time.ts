const ZONE_SUFFIX = /(Z|[+-]\d{2}:?\d{2})$/;

const YEAR_SECONDS = 31536000;
const DAY_SECONDS = 86400;
const HOUR_SECONDS = 3600;
const MINUTE_SECONDS = 60;

const DEFAULT_LOCALE = "en-GB";
const EXACT_DATE_TIME_OPTIONS: Intl.DateTimeFormatOptions = { dateStyle: "long", timeStyle: "short" };
const EXACT_DATE_TIME_FORMAT = new Intl.DateTimeFormat(DEFAULT_LOCALE, EXACT_DATE_TIME_OPTIONS);

export function parseServerDate(input: string | null | undefined): Date | null {
    if (!input) {
        return null;
    }

    const trimmed = input.trim();
    if (trimmed === "") {
        return null;
    }

    const normalised = trimmed.replace(" ", "T");
    const zoned = ZONE_SUFFIX.test(normalised) ? normalised : `${normalised}Z`;

    const parsed = new Date(zoned);
    if (Number.isNaN(parsed.getTime())) {
        return null;
    }

    return parsed;
}

export function formatDuration(ms: number): string {
    const totalSeconds = Math.round(ms / 1000);
    const levels: [number, string][] = [
        [Math.floor(totalSeconds / YEAR_SECONDS), "years"],
        [Math.floor((totalSeconds % YEAR_SECONDS) / DAY_SECONDS), "days"],
        [Math.floor((totalSeconds % DAY_SECONDS) / HOUR_SECONDS), "hours"],
        [Math.floor((totalSeconds % HOUR_SECONDS) / MINUTE_SECONDS), "minutes"],
        [totalSeconds % MINUTE_SECONDS, "seconds"],
    ];

    let result = "";
    for (const [value, label] of levels) {
        if (value === 0) {
            continue;
        }

        result += ` ${value} ${value === 1 ? label.slice(0, -1) : label}`;
    }

    return result.trim() || "0 seconds";
}

export function relativeTime(dateStr: string | null | undefined, style: "long" | "short" = "long"): string {
    const parsed = parseServerDate(dateStr);
    if (!parsed) {
        return "";
    }

    const seconds = Math.floor((Date.now() - parsed.getTime()) / 1000);
    if (seconds < 60) {
        return "just now";
    }

    const suffix = style === "long" ? " ago" : "";

    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
        return `${minutes}m${suffix}`;
    }

    const hours = Math.floor(minutes / 60);
    if (hours < 24) {
        return `${hours}h${suffix}`;
    }

    const days = Math.floor(hours / 24);
    if (days < 30) {
        return `${days}d${suffix}`;
    }

    const months = Math.floor(days / 30);

    return `${months}mo${suffix}`;
}

export function formatDate(dateStr: string | null | undefined, locale: string | string[] = DEFAULT_LOCALE): string {
    const parsed = parseServerDate(dateStr);
    if (!parsed) {
        return "";
    }

    return parsed.toLocaleDateString(locale);
}

export function formatExactDateTime(
    dateStr: string | null | undefined,
    locale: string | string[] = DEFAULT_LOCALE,
): string {
    const parsed = parseServerDate(dateStr);
    if (!parsed) {
        return "";
    }

    if (locale === DEFAULT_LOCALE) {
        return EXACT_DATE_TIME_FORMAT.format(parsed);
    }

    return new Intl.DateTimeFormat(locale, EXACT_DATE_TIME_OPTIONS).format(parsed);
}
