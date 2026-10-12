import { Suspense, useEffect, useId, useState } from "react";
import { Link, NavLink, Outlet } from "react-router";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { Logo } from "../../../components/Logo/Logo";
import { SessionExpiredBanner } from "../../../components/SessionExpiredBanner/SessionExpiredBanner";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { ADMIN_HOME_PATH, appReturnPath, groupedAdminSections } from "../../../domain/adminSections";
import { useAuth } from "../../../hooks/useAuth";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { AccountMenu } from "./AccountMenu";
import styles from "./StaffConsoleLayout.module.css";

const NAV_ID = "admin-console-nav";
const MAIN_ID = "admin-main";

function linkClass({ isActive }: { isActive: boolean }): string {
    return isActive ? `${styles.navLink} ${styles.navLinkActive}` : styles.navLink;
}

export function StaffConsoleLayout() {
    const { permissions } = useAuth();
    const { site_name, site_mode } = useSiteInfo();
    const { current } = useCurrentOrg();
    const groupIdPrefix = useId();
    const [navOpen, setNavOpen] = useState(false);

    const groups = groupedAdminSections(permissions, site_mode);
    const backTo = appReturnPath(current);

    useEffect(() => {
        if (!navOpen) {
            return;
        }

        function handleKeyDown(event: KeyboardEvent) {
            if (event.key === "Escape") {
                setNavOpen(false);
            }
        }

        document.addEventListener("keydown", handleKeyDown);

        return () => {
            document.removeEventListener("keydown", handleKeyDown);
        };
    }, [navOpen]);

    const railClass = navOpen ? `${styles.rail} ${styles.railOpen}` : styles.rail;

    return (
        <div className={styles.console}>
            <a href={`#${MAIN_ID}`} className="skip-link">
                Skip to content
            </a>

            <header className={styles.topBar}>
                <button
                    type="button"
                    className={styles.menuButton}
                    aria-label="Admin menu"
                    aria-controls={NAV_ID}
                    aria-expanded={navOpen}
                    onClick={() => setNavOpen(open => !open)}
                >
                    <span className={styles.menuBar} />
                    <span className={styles.menuBar} />
                    <span className={styles.menuBar} />
                </button>

                <Link
                    to={ADMIN_HOME_PATH}
                    className={styles.identity}
                    aria-label={`${site_name} Admin console`}
                    onClick={() => setNavOpen(false)}
                >
                    <Logo className={styles.logo} />
                    <span className={styles.identityName}>Admin console</span>
                </Link>

                <div className={styles.topActions}>
                    <ButtonLink to={backTo} size="small" className={styles.backLink}>
                        Back to app
                    </ButtonLink>
                    <AccountMenu backTo={backTo} />
                </div>
            </header>

            {navOpen && (
                <button
                    type="button"
                    className={styles.overlay}
                    aria-label="Close menu"
                    onClick={() => setNavOpen(false)}
                />
            )}

            <aside id={NAV_ID} className={railClass} aria-label="Admin console">
                <nav className={styles.nav} aria-label="Admin sections">
                    {groups.map(view => {
                        const labelId = `${groupIdPrefix}-${view.group}`;

                        return (
                            <div key={view.group} className={styles.navGroup}>
                                <span id={labelId} className={styles.navGroupLabel}>
                                    {view.label}
                                </span>
                                <ul className={styles.navList} aria-labelledby={labelId}>
                                    {view.sections.map(section => (
                                        <li key={section.path}>
                                            <NavLink
                                                to={section.path}
                                                end={section.path === ADMIN_HOME_PATH}
                                                className={linkClass}
                                                onClick={() => setNavOpen(false)}
                                            >
                                                {section.label}
                                            </NavLink>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                        );
                    })}
                </nav>

                <div className={styles.railFooter}>
                    <span className={styles.siteName}>
                        <bdi>{site_name}</bdi>
                    </span>
                    <StatusTag tone="neutral">{site_mode === "private" ? "Private site" : "Public site"}</StatusTag>
                </div>
            </aside>

            <main id={MAIN_ID} tabIndex={-1} className={styles.main}>
                <SessionExpiredBanner />
                <h1 className="visually-hidden">Admin console</h1>
                <div className={styles.content}>
                    <Suspense fallback={<div className="loading">Loading...</div>}>
                        <Outlet />
                    </Suspense>
                </div>
            </main>
        </div>
    );
}
