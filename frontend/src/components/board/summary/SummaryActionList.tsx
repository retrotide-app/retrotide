import { ACTION_ITEM_STATUS_LABELS, formatDueDate } from "../../../domain/board/actionItems";
import type { ActionItemStatus, ActionItemView } from "../../../types/api";
import { StatusTag, type StatusTone } from "../../StatusTag/StatusTag";
import { MentionText } from "../mentions/MentionText";
import styles from "./BoardSummary.module.css";

interface SummaryActionListProps {
    label: string;
    items: readonly ActionItemView[];
}

const STATUS_TONES: Readonly<Record<ActionItemStatus, StatusTone>> = {
    open: "warning",
    done: "success",
    dropped: "neutral",
};

export function SummaryActionList({ label, items }: SummaryActionListProps) {
    return (
        <ul className={styles.actions} aria-label={label}>
            {items.map(item => {
                const due = formatDueDate(item.due_on);

                return (
                    <li key={item.id} className={styles.action}>
                        <span className={styles.actionTitle}>
                            <MentionText text={item.title} mentions={item.mentions} />
                        </span>
                        <span className={styles.meta}>
                            <span>
                                {item.assignee === null ? "No one assigned" : <bdi>{item.assignee.display_name}</bdi>}
                            </span>
                            {due !== "" && <span>Due {due}</span>}
                            <StatusTag tone={STATUS_TONES[item.status]}>
                                {ACTION_ITEM_STATUS_LABELS[item.status]}
                            </StatusTag>
                        </span>
                    </li>
                );
            })}
        </ul>
    );
}
