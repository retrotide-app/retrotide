import type { ComponentProps } from "react";
import { buttonClassName, type ButtonSize, type ButtonVariant } from "./buttonClassName";

interface ButtonProps extends ComponentProps<"button"> {
    variant?: ButtonVariant;
    size?: ButtonSize;
}

export function Button({ variant = "secondary", size = "medium", className, children, ...rest }: ButtonProps) {
    return (
        <button className={buttonClassName(variant, size, className)} {...rest}>
            {children}
        </button>
    );
}
