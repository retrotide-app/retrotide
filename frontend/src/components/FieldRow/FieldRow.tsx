import type { PropsWithChildren } from "react";
import styles from "./FieldRow.module.css";

export function FieldRow({ children }: PropsWithChildren) {
    return <div className={styles.row}>{children}</div>;
}
