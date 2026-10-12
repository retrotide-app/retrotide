import type { SelectHTMLAttributes } from "react";
import styles from "./Select.module.css";

interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, "size"> {
    size?: "small" | "medium";
}

export function Select({ size = "medium", className, children, ...rest }: SelectProps) {
    const classes = [styles.select, styles[size], className].filter(Boolean).join(" ");

    return (
        <select className={classes} {...rest}>
            {children}
        </select>
    );
}
