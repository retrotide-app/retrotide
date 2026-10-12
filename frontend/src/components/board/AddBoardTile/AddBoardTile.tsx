import { Link } from "react-router";
import styles from "./AddBoardTile.module.css";

interface AddBoardTileProps {
    href: string;
    label: string;
}

export function AddBoardTile({ href, label }: AddBoardTileProps) {
    return (
        <Link to={href} className={styles.tile}>
            <span className={styles.plus} aria-hidden="true">
                +
            </span>
            <span className={styles.label}>{label}</span>
        </Link>
    );
}
