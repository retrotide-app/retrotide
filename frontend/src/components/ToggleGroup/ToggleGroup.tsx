import type { ReactNode } from "react";
import { Button } from "../Button/Button";
import styles from "./ToggleGroup.module.css";

export interface ToggleOption<T extends string> {
    value: T;
    label: ReactNode;
}

interface ToggleGroupProps<T extends string> {
    label: string;
    options: readonly ToggleOption<T>[];
    value: T;
    onChange: (value: T) => void;
}

export function ToggleGroup<T extends string>({ label, options, value, onChange }: ToggleGroupProps<T>) {
    return (
        <div role="group" aria-label={label} className={styles.group}>
            {options.map(option => {
                const current = option.value === value;

                return (
                    <Button
                        key={option.value}
                        type="button"
                        size="small"
                        variant={current ? "primary" : "secondary"}
                        aria-pressed={current}
                        onClick={() => onChange(option.value)}
                    >
                        {option.label}
                    </Button>
                );
            })}
        </div>
    );
}
