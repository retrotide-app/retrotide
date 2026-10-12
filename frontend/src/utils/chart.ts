export interface ChartPoint {
    date: string;
    value: number;
}

export interface Plural {
    one: string;
    other: string;
}

export const COUNT_FORMAT = new Intl.NumberFormat("en-GB");
const SHORT_DAY_FORMAT = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });
const LONG_DAY_FORMAT = new Intl.DateTimeFormat("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "long",
    timeZone: "UTC",
});
const NICE_STEPS = [1, 2, 5, 10];
const TARGET_TICKS = 4;

export function countOf(value: number, noun: Plural): string {
    return `${COUNT_FORMAT.format(value)} ${value === 1 ? noun.one : noun.other}`;
}

function dayOf(date: string): Date {
    const parsed = new Date(`${date}T00:00:00Z`);

    if (Number.isNaN(parsed.getTime())) {
        throw new Error(`invalid chart date "${date}"`);
    }

    return parsed;
}

export function formatShortDay(date: string): string {
    return SHORT_DAY_FORMAT.format(dayOf(date));
}

export function formatLongDay(date: string): string {
    return LONG_DAY_FORMAT.format(dayOf(date));
}

export function niceTicks(max: number): number[] {
    const raw = Math.max(max, 1) / TARGET_TICKS;
    const magnitude = 10 ** Math.floor(Math.log10(raw));
    const multiple = NICE_STEPS.find(step => step * magnitude >= raw) ?? 10;
    const step = Math.max(1, multiple * magnitude);
    const top = Math.max(step, Math.ceil(max / step) * step);

    const ticks: number[] = [];
    for (let tick = 0; tick <= top; tick += step) {
        ticks.push(tick);
    }

    return ticks;
}

export function weeklyLabelIndices(count: number): number[] {
    const indices: number[] = [];

    for (let index = count - 1; index >= 0; index -= 7) {
        indices.unshift(index);
    }

    return indices;
}

export function seriesTotal(points: readonly ChartPoint[]): number {
    return points.reduce((sum, point) => sum + point.value, 0);
}

export function peakPoint(points: readonly ChartPoint[]): ChartPoint | null {
    return points.reduce<ChartPoint | null>((peak, point) => {
        if (peak === null || point.value > peak.value) {
            return point;
        }

        return peak;
    }, null);
}

export function seriesSummary(points: readonly ChartPoint[], noun: Plural): string {
    const total = seriesTotal(points);
    const peak = peakPoint(points);
    const period = `${countOf(total, noun)} in the last ${points.length} days.`;

    if (peak === null || peak.value === 0) {
        return period;
    }

    return `${period} Busiest day: ${formatLongDay(peak.date)}, with ${COUNT_FORMAT.format(peak.value)}.`;
}

export function pointPercent(index: number, count: number): number {
    if (count <= 1) {
        return 50;
    }

    return (index / (count - 1)) * 100;
}

export function valuePercent(value: number, top: number): number {
    if (top <= 0) {
        return 100;
    }

    return 100 - (value / top) * 100;
}

export function linePath(points: readonly ChartPoint[], top: number): string {
    return points
        .map((point, index) => {
            const command = index === 0 ? "M" : "L";

            return `${command}${pointPercent(index, points.length)} ${valuePercent(point.value, top)}`;
        })
        .join(" ");
}

export function areaPath(points: readonly ChartPoint[], top: number): string {
    if (points.length === 0) {
        return "";
    }

    return `${linePath(points, top)} L${pointPercent(points.length - 1, points.length)} 100 L${pointPercent(0, points.length)} 100 Z`;
}

export function nearestIndex(ratio: number, count: number): number {
    if (count <= 1) {
        return 0;
    }

    const clamped = Math.min(1, Math.max(0, ratio));

    return Math.round(clamped * (count - 1));
}
