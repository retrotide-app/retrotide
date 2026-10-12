import type { ReactNode } from "react";
import { Link } from "react-router";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { Logo } from "../../Logo/Logo";
import styles from "./AuthLayout.module.css";

interface AuthLayoutProps {
    title: string;
    children: ReactNode;
    footer?: ReactNode;
}

export function AuthLayout({ title, children, footer }: AuthLayoutProps) {
    const { site_name } = useSiteInfo();

    return (
        <main id="main-content" tabIndex={-1} className={styles.page}>
            <div className={styles.card}>
                <Link to="/" className={styles.brand}>
                    <Logo label={site_name} className={styles.logo} />
                </Link>
                <h1 className={styles.title}>{title}</h1>
                {children}
            </div>
            {footer !== undefined && <div className={styles.footer}>{footer}</div>}
        </main>
    );
}
