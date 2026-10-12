import { useId } from "react";
import { formatAverage, resultRows, sharePercent } from "../../../domain/board/surveys";
import type { SurveyQuestionView } from "../../../types/api";
import styles from "./Surveys.module.css";

interface SurveyResultsProps {
    question: SurveyQuestionView;
}

export function SurveyResults({ question }: SurveyResultsProps) {
    const promptId = useId();
    const rows = resultRows(question);

    return (
        <section aria-labelledby={promptId} className={styles.results}>
            <p id={promptId} className={styles.prompt} dir="auto">
                {question.prompt}
            </p>
            <ul className={styles.resultList}>
                {rows.map((row, index) => (
                    <li
                        key={`${index}:${row.label}`}
                        aria-label={`${row.label}: ${row.count}`}
                        className={styles.resultRow}
                    >
                        <span className={styles.resultLabel} dir="auto">
                            {row.label}
                        </span>
                        <span className={styles.track} aria-hidden="true">
                            <span className={styles.bar} style={{ width: `${sharePercent(row)}%` }} />
                        </span>
                        <span className={styles.resultCount}>{row.count}</span>
                    </li>
                ))}
            </ul>
            {question.kind === "scale" && question.results !== null && (
                <p className={styles.average}>Average: {formatAverage(question.results.average)}</p>
            )}
        </section>
    );
}
