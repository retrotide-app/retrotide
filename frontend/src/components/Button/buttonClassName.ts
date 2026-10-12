import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";
export type ButtonSize = "small" | "medium" | "large";

export function buttonClassName(variant: ButtonVariant, size: ButtonSize, className?: string): string {
    return [styles.button, styles[variant], styles[size], className].filter(Boolean).join(" ");
}
