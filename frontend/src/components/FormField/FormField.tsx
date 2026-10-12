import { type ReactNode, useId } from "react";
import styles from "./FormField.module.css";

export type FieldLayout = "stacked" | "row";

export interface FieldControlProps {
    id: string;
    "aria-describedby"?: string;
    "aria-invalid"?: true;
}

interface FormFieldProps {
    label: string;
    hint?: ReactNode;
    error?: string | null;
    layout?: FieldLayout;
    flush?: boolean;
    children: (control: FieldControlProps) => ReactNode;
}

export function FormField({ label, hint, error, layout = "stacked", flush = false, children }: FormFieldProps) {
    const id = useId();
    const hintId = `${id}-hint`;
    const errorId = `${id}-error`;
    const hasHint = hint !== undefined && hint !== null && hint !== "";
    const hasError = error !== undefined && error !== null && error !== "";

    const describedBy = [hasHint ? hintId : "", hasError ? errorId : ""].filter(Boolean).join(" ");

    const control: FieldControlProps = {
        id,
        "aria-describedby": describedBy === "" ? undefined : describedBy,
        "aria-invalid": hasError ? true : undefined,
    };

    return (
        <div
            className={[styles.field, layout === "row" ? styles.row : "", flush ? styles.flush : ""]
                .filter(Boolean)
                .join(" ")}
        >
            <div className={styles.text}>
                <label htmlFor={id} className={styles.label}>
                    {label}
                </label>
                {hasHint && (
                    <p id={hintId} className={styles.hint}>
                        {hint}
                    </p>
                )}
            </div>
            <div className={styles.control}>
                {hasError && (
                    <p id={errorId} className={styles.error} dir="auto">
                        {error}
                    </p>
                )}
                {children(control)}
            </div>
        </div>
    );
}
