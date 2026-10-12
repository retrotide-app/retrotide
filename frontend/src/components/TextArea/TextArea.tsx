import type { ComponentProps } from "react";
import styles from "./TextArea.module.css";

type TextAreaProps = ComponentProps<"textarea"> & {
    compact?: boolean;
};

export function TextArea({ className, compact = false, ...rest }: TextAreaProps) {
    const classes = [styles.textarea, compact ? styles.compact : "", className].filter(Boolean).join(" ");

    return <textarea dir="auto" className={classes} {...rest} />;
}
