import { useId } from "react";
import { onlineRows, participantRows } from "../../domain/board/facilitation";
import type { BoardSnapshot } from "../../types/api";
import { ParticipantList } from "./ParticipantList";
import styles from "./PeoplePanel.module.css";

interface PeoplePanelProps {
    snapshot: BoardSnapshot;
    online: ReadonlySet<string> | null;
}

export function PeoplePanel({ snapshot, online }: PeoplePanelProps) {
    const headingId = useId();
    const rows = onlineRows(participantRows(snapshot), online);

    return (
        <aside aria-labelledby={headingId} className={styles.panel} data-rail="people">
            <h2 id={headingId} className={styles.heading}>
                Here now <span className={styles.count}>{rows.length}</span>
            </h2>
            <ParticipantList snapshot={snapshot} rows={rows} plain />
        </aside>
    );
}
