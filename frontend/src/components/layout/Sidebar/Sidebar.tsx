import { NavLink } from "react-router";
import {
    ORGANISATION_SECTION_LABELS,
    organisationHomePath,
    organisationPath,
    visibleOrganisationSections,
} from "../../../domain/organisations";
import { useAuth } from "../../../hooks/useAuth";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { OrgSwitcher } from "../OrgSwitcher/OrgSwitcher";
import styles from "./Sidebar.module.css";

interface SidebarProps {
    open: boolean;
    onClose: () => void;
}

interface SidebarLink {
    to: string;
    label: string;
    end?: boolean;
}

function linkClass({ isActive }: { isActive: boolean }): string {
    return isActive ? `${styles.link} ${styles.active}` : styles.link;
}

export function Sidebar({ open, onClose }: SidebarProps) {
    const siteInfo = useSiteInfo();
    const { user, permissions } = useAuth();
    const { current } = useCurrentOrg();

    const inOrganisation = user !== null && current !== null;
    const home = inOrganisation ? organisationHomePath(current.slug) : "/";
    const links: SidebarLink[] = [{ to: home, label: "Home", end: true }];

    if (inOrganisation) {
        for (const section of visibleOrganisationSections(current)) {
            links.push({ to: organisationPath(current.slug, section), label: ORGANISATION_SECTION_LABELS[section] });
        }
    }

    if (user !== null) {
        links.push({ to: "/account", label: "Account" });
    }

    if (user !== null && permissions.includes("view_admin_panel")) {
        links.push({ to: "/admin", label: "Admin" });
    }

    const sidebarClass = open ? `${styles.sidebar} ${styles.open}` : styles.sidebar;

    return (
        <>
            {open && <button type="button" className={styles.overlay} aria-label="Close menu" onClick={onClose} />}
            <aside id="app-sidebar" className={sidebarClass} aria-label="Main navigation">
                <NavLink to="/" className={styles.brand} onClick={onClose}>
                    <img src="/favicon.svg" alt="" width={36} height={36} className={styles.brandMark} />
                    <span className={styles.brandName}>{siteInfo.site_name}</span>
                </NavLink>

                {user !== null && <OrgSwitcher />}

                <nav className={styles.nav} aria-label="Primary">
                    {links.map(link => (
                        <NavLink key={link.to} to={link.to} end={link.end} className={linkClass} onClick={onClose}>
                            {link.label}
                        </NavLink>
                    ))}
                </nav>
            </aside>
        </>
    );
}
