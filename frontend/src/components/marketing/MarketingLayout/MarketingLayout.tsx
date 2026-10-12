import type { PropsWithChildren } from "react";
import { Link } from "react-router";
import {
    COMPARE_EASYRETRO_PATH,
    LEGAL_LINKS,
    legalPath,
    PRICING_PATH,
    SELF_HOST_PATH,
    TEMPLATES_PATH,
} from "../../../domain/marketing";
import { ButtonLink } from "../../Button/ButtonLink";
import { ThemeSelect } from "../../layout/ThemeSelect/ThemeSelect";
import { TextLink } from "../../TextLink/TextLink";
import styles from "./MarketingLayout.module.css";

const COPYRIGHT_YEAR = new Date().getFullYear();

const SITE_LINKS = [
    { to: TEMPLATES_PATH, label: "Templates" },
    { to: PRICING_PATH, label: "Pricing" },
    { to: COMPARE_EASYRETRO_PATH, label: "Compare" },
    { to: SELF_HOST_PATH, label: "Self-host" },
];

const FOOTER_GROUPS = [
    {
        heading: "Product",
        links: [
            { to: TEMPLATES_PATH, label: "Templates" },
            { to: PRICING_PATH, label: "Pricing" },
            { to: COMPARE_EASYRETRO_PATH, label: "Compare with EasyRetro" },
            { to: SELF_HOST_PATH, label: "Self-host" },
        ],
    },
    {
        heading: "Account",
        links: [
            { to: "/login", label: "Sign in" },
            { to: "/register", label: "Create an account" },
        ],
    },
    {
        heading: "Legal",
        links: LEGAL_LINKS.map(link => ({ to: legalPath(link.page), label: link.label })),
    },
];

export function MarketingLayout({ children }: PropsWithChildren) {
    return (
        <div className={styles.layout}>
            <a href="#main-content" className="skip-link">
                Skip to content
            </a>
            <header className={styles.header}>
                <div className={styles.bar}>
                    <Link to="/" className={styles.brand}>
                        <img src="/favicon.svg" alt="" className={styles.mark} />
                        <span className={styles.wordmark}>
                            Retro<span className={styles.wordmarkTide}>Tide</span>
                        </span>
                    </Link>
                    <nav aria-label="Site" className={styles.sections}>
                        {SITE_LINKS.map(link => (
                            <TextLink key={link.to} to={link.to} tone="inverse">
                                {link.label}
                            </TextLink>
                        ))}
                    </nav>
                    <nav aria-label="Account" className={styles.account}>
                        <TextLink to="/login" tone="inverse">
                            Sign in
                        </TextLink>
                        <ButtonLink to="/register" variant="primary" size="small">
                            Start for free
                        </ButtonLink>
                    </nav>
                </div>
            </header>
            <main id="main-content" tabIndex={-1} className={styles.main}>
                {children}
            </main>
            <footer className={styles.footer}>
                <div className={styles.footerInner}>
                    <nav aria-label="Footer" className={styles.footerNav}>
                        {FOOTER_GROUPS.map(group => (
                            <div key={group.heading} className={styles.footerGroup}>
                                <p className={styles.footerHeading}>{group.heading}</p>
                                {group.links.map(link => (
                                    <TextLink key={link.to} to={link.to} tone="muted">
                                        {link.label}
                                    </TextLink>
                                ))}
                            </div>
                        ))}
                    </nav>
                    <div className={styles.footerBase}>
                        <p>© {COPYRIGHT_YEAR} RetroTide</p>
                        <ThemeSelect />
                    </div>
                </div>
            </footer>
        </div>
    );
}
