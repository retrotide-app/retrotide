import type { AnchorHTMLAttributes } from "react";
import { Link } from "react-router";
import { buttonClassName, type ButtonSize, type ButtonVariant } from "./buttonClassName";

type Destination = { to: string; state?: unknown; href?: never } | { href: string; to?: never; state?: never };

type ButtonLinkProps = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
    Destination & {
        variant?: ButtonVariant;
        size?: ButtonSize;
    };

export function ButtonLink({
    to,
    state,
    href,
    variant = "secondary",
    size = "medium",
    className,
    children,
    ...rest
}: ButtonLinkProps) {
    const classes = buttonClassName(variant, size, className);

    if (to !== undefined) {
        return (
            <Link to={to} state={state} className={classes} {...rest}>
                {children}
            </Link>
        );
    }

    return (
        <a href={href} className={classes} {...rest}>
            {children}
        </a>
    );
}
