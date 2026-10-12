import type { AnchorHTMLAttributes } from "react";
import { Link } from "react-router";
import styles from "./TextLink.module.css";

type Destination = { to: string; state?: unknown; href?: never } | { href: string; to?: never; state?: never };

type TextLinkProps = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
    Destination & {
        tone?: "default" | "muted" | "inverse";
    };

export function TextLink({ to, state, href, tone = "default", className, children, ...rest }: TextLinkProps) {
    const classes = [styles.link, styles[tone], className].filter(Boolean).join(" ");

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
