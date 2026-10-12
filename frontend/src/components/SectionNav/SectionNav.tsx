import { Link } from "react-router";
import styles from "./SectionNav.module.css";

export interface SectionNavItem {
    id: string;
    label: string;
    to: string;
}

interface SectionNavProps {
    label: string;
    items: readonly SectionNavItem[];
    current: string;
}

export function SectionNav({ label, items, current }: SectionNavProps) {
    return (
        <nav aria-label={label} className={styles.nav}>
            <ul className={styles.list}>
                {items.map(item => (
                    <li key={item.id}>
                        <Link
                            to={item.to}
                            className={item.id === current ? `${styles.link} ${styles.current}` : styles.link}
                            aria-current={item.id === current ? "page" : undefined}
                        >
                            {item.label}
                        </Link>
                    </li>
                ))}
            </ul>
        </nav>
    );
}
