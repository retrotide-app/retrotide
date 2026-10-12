import { type KeyboardEvent, type PointerEvent, useId, useState } from "react";
import {
    areaPath,
    type ChartPoint,
    COUNT_FORMAT,
    countOf,
    formatLongDay,
    formatShortDay,
    linePath,
    nearestIndex,
    niceTicks,
    type Plural,
    pointPercent,
    seriesSummary,
    seriesTotal,
    valuePercent,
    weeklyLabelIndices,
} from "../../utils/chart";
import { DataTable } from "../DataTable/DataTable";
import styles from "./DailyChart.module.css";

export type DailyChartHeadingLevel = 2 | 3 | 4;

type ActiveSource = "keyboard" | "pointer";

interface DailyChartProps {
    title: string;
    headingLevel: DailyChartHeadingLevel;
    points: readonly ChartPoint[];
    noun: Plural;
    size?: "large" | "compact";
}

const STEP_KEYS: Readonly<Record<string, number>> = { ArrowLeft: -1, ArrowRight: 1 };

const HEADING_TAGS = { 2: "h2", 3: "h3", 4: "h4" } as const;

export function DailyChart({ title, headingLevel, points, noun, size = "compact" }: DailyChartProps) {
    const titleId = useId();
    const summaryId = useId();
    const hintId = useId();
    const [active, setActive] = useState<number | null>(null);
    const [source, setSource] = useState<ActiveSource>("pointer");

    const ticks = niceTicks(Math.max(0, ...points.map(point => point.value)));
    const top = ticks[ticks.length - 1] ?? 1;
    const last = points.length - 1;
    const labelled = new Set(weeklyLabelIndices(points.length));
    const activePoint = active === null ? undefined : points[active];
    const activeX = active === null ? 0 : pointPercent(active, points.length);

    function handlePointerMove(event: PointerEvent<HTMLDivElement>) {
        const rect = event.currentTarget.getBoundingClientRect();

        if (rect.width <= 0) {
            return;
        }

        setSource("pointer");
        setActive(nearestIndex((event.clientX - rect.left) / rect.width, points.length));
    }

    function handleFocus() {
        if (active !== null || last < 0) {
            return;
        }

        setSource("keyboard");
        setActive(last);
    }

    function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
        if (points.length === 0) {
            return;
        }

        const step = STEP_KEYS[event.key];
        let next: number | null = null;

        if (step !== undefined) {
            next = Math.min(last, Math.max(0, (active ?? last) + step));
        }

        if (event.key === "Home") {
            next = 0;
        }

        if (event.key === "End") {
            next = last;
        }

        if (next === null) {
            return;
        }

        event.preventDefault();
        setSource("keyboard");
        setActive(next);
    }

    const Heading = HEADING_TAGS[headingLevel];
    const chartClass = size === "large" ? `${styles.chart} ${styles.large}` : styles.chart;
    const tooltipClass = activeX > 50 ? `${styles.tooltip} ${styles.tooltipLeft}` : styles.tooltip;

    return (
        <section aria-labelledby={titleId} className={chartClass}>
            <div className={styles.header}>
                <Heading id={titleId} className={styles.title}>
                    {title}
                </Heading>
                <p className={styles.total}>{COUNT_FORMAT.format(seriesTotal(points))}</p>
            </div>
            <p id={summaryId} className={styles.summary}>
                {seriesSummary(points, noun)}
            </p>

            <div className={styles.frame}>
                <div className={styles.yAxis} aria-hidden="true">
                    {ticks.map(tick => (
                        <span key={tick} className={styles.yTick} style={{ top: `${valuePercent(tick, top)}%` }}>
                            {COUNT_FORMAT.format(tick)}
                        </span>
                    ))}
                </div>

                <div
                    className={styles.plot}
                    role="group"
                    tabIndex={0}
                    aria-label={`${title}, day by day`}
                    aria-describedby={`${summaryId} ${hintId}`}
                    onPointerMove={handlePointerMove}
                    onPointerLeave={() => setActive(null)}
                    onKeyDown={handleKeyDown}
                    onFocus={handleFocus}
                    onBlur={() => setActive(null)}
                >
                    <svg
                        className={styles.svg}
                        viewBox="0 0 100 100"
                        preserveAspectRatio="none"
                        aria-hidden="true"
                        focusable="false"
                    >
                        {ticks.map(tick => (
                            <line
                                key={tick}
                                className={tick === 0 ? styles.baseline : styles.grid}
                                x1={0}
                                x2={100}
                                y1={valuePercent(tick, top)}
                                y2={valuePercent(tick, top)}
                                vectorEffect="non-scaling-stroke"
                            />
                        ))}
                        <path className={styles.area} d={areaPath(points, top)} />
                        <path className={styles.line} d={linePath(points, top)} vectorEffect="non-scaling-stroke" />
                    </svg>

                    {activePoint !== undefined && (
                        <div className={styles.hover} aria-hidden="true">
                            <span className={styles.crosshair} style={{ left: `${activeX}%` }} />
                            <span
                                className={styles.dot}
                                style={{ left: `${activeX}%`, top: `${valuePercent(activePoint.value, top)}%` }}
                            />
                            <span className={tooltipClass} style={{ left: `${activeX}%` }}>
                                <strong className={styles.tooltipValue}>
                                    {COUNT_FORMAT.format(activePoint.value)}
                                </strong>
                                <span className={styles.tooltipDate}>{formatLongDay(activePoint.date)}</span>
                            </span>
                        </div>
                    )}
                </div>

                <div className={styles.xAxis} aria-hidden="true">
                    {points.map((point, index) => {
                        if (!labelled.has(index)) {
                            return null;
                        }

                        const x = pointPercent(index, points.length);

                        return (
                            <span
                                key={point.date}
                                className={styles.xTick}
                                style={{ left: `${x}%`, transform: `translateX(-${x}%)` }}
                            >
                                {formatShortDay(point.date)}
                            </span>
                        );
                    })}
                </div>
            </div>

            <p id={hintId} className="visually-hidden">
                Use the left and right arrow keys to read each day.
            </p>
            <p className="visually-hidden" aria-live="polite">
                {activePoint === undefined || source !== "keyboard"
                    ? ""
                    : `${formatLongDay(activePoint.date)}: ${countOf(activePoint.value, noun)}`}
            </p>

            <details className={styles.details}>
                <summary className={styles.detailsToggle}>Show the numbers</summary>
                <DataTable caption={`${title} per day`}>
                    <thead>
                        <tr>
                            <th scope="col">Day</th>
                            <th scope="col" className={styles.numberCell}>
                                {title}
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        {points.map(point => (
                            <tr key={point.date}>
                                <td>{formatLongDay(point.date)}</td>
                                <td className={styles.numberCell}>{COUNT_FORMAT.format(point.value)}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            </details>
        </section>
    );
}
