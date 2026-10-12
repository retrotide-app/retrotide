import styles from "./StepProgress.module.css";

interface StepProgressProps {
    label: string;
    steps: readonly string[];
    current: number;
    compact?: boolean;
}

function stepClass(index: number, current: number): string {
    if (index === current) {
        return `${styles.step} ${styles.current}`;
    }

    if (index < current) {
        return `${styles.step} ${styles.done}`;
    }

    return styles.step;
}

export function StepProgress({ label, steps, current, compact = false }: StepProgressProps) {
    return (
        <ol className={compact ? `${styles.steps} ${styles.compact}` : styles.steps} aria-label={label}>
            {steps.map((step, index) => (
                <li
                    key={step}
                    className={stepClass(index, current)}
                    aria-current={index === current ? "step" : undefined}
                >
                    <span className={styles.number} aria-hidden="true">
                        {index + 1}
                    </span>
                    <span className={styles.name}>
                        <span className="visually-hidden">{`Step ${index + 1} of ${steps.length}: `}</span>
                        {step}
                    </span>
                </li>
            ))}
        </ol>
    );
}
