import { useId } from "react";
import {
    ACTION_ITEM_STATUS_LABELS,
    formatDueDate,
    REVIEW_OUTCOME_LABELS,
    REVIEW_OUTCOMES,
} from "../../../domain/board/actionItems";
import { actionItemActions, isArchived, NO_ACTION_ITEM_ACTIONS } from "../../../domain/board/capabilities";
import { useBoardMembership } from "../../../hooks/board/useBoardMembership";
import { useReviewPreviousAction } from "../../../hooks/mutations/actionItems";
import type { BoardSnapshot, PreviousActionView, ReviewOutcome } from "../../../types/api";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { StatusTag } from "../../StatusTag/StatusTag";
import { Toast } from "../../Toast/Toast";
import { type ToggleOption, ToggleGroup } from "../../ToggleGroup/ToggleGroup";
import { MentionText } from "../mentions/MentionText";
import styles from "./ActionItems.module.css";

interface PreviousActionsPanelProps {
    snapshot: BoardSnapshot;
    writable: boolean;
}

interface PreviousActionRowProps {
    item: PreviousActionView;
    reviews: boolean;
    onReview: (outcome: ReviewOutcome) => void;
}

type ReviewChoice = ReviewOutcome | "none";

const REVIEW_CHOICES: readonly ToggleOption<ReviewChoice>[] = [
    { value: "done", label: "Done" },
    { value: "carried", label: "Carry over" },
    { value: "dropped", label: "Drop" },
];

function isOutcome(choice: ReviewChoice): choice is ReviewOutcome {
    return REVIEW_OUTCOMES.some(outcome => outcome === choice);
}

function PreviousActionRow({ item, reviews, onReview }: PreviousActionRowProps) {
    const titleId = useId();
    const due = formatDueDate(item.due_on);

    return (
        <li className={styles.item} aria-labelledby={titleId}>
            <div className={styles.itemMain}>
                <span className={styles.itemTitle}>
                    <MentionText id={titleId} text={item.title} mentions={item.mentions} />
                </span>
                <span className={styles.itemMeta}>
                    {item.assignee === null ? "No one assigned" : <bdi>{item.assignee.display_name}</bdi>}
                    {due !== "" && ` · Due ${due}`}
                </span>
            </div>
            <div className={styles.itemSide}>
                {item.review === null ? (
                    <StatusTag tone="warning">{ACTION_ITEM_STATUS_LABELS[item.status]}</StatusTag>
                ) : (
                    <StatusTag tone={item.review === "done" ? "success" : "neutral"}>
                        {REVIEW_OUTCOME_LABELS[item.review]}
                    </StatusTag>
                )}
                {reviews && (
                    <ToggleGroup
                        label={`Review ${item.title}`}
                        options={REVIEW_CHOICES}
                        value={item.review ?? "none"}
                        onChange={choice => {
                            if (isOutcome(choice) && choice !== item.review) {
                                onReview(choice);
                            }
                        }}
                    />
                )}
            </div>
        </li>
    );
}

export function PreviousActionsPanel({ snapshot, writable }: PreviousActionsPanelProps) {
    const headingId = useId();
    const { board, board_id: boardId, previous_actions: items } = snapshot;
    const membership = useBoardMembership(boardId, writable && !isArchived(board) && items.length > 0);
    const review = useReviewPreviousAction(boardId);

    if (items.length === 0) {
        return null;
    }

    const actions = writable ? actionItemActions(snapshot, membership.member) : NO_ACTION_ITEM_ACTIONS;
    const waiting = items.filter(item => item.review === null).length;

    return (
        <section aria-labelledby={headingId} className={styles.panel}>
            <div className={styles.header}>
                <h2 id={headingId} className={styles.heading}>
                    Previous actions <span className={styles.count}>{items.length}</span>
                </h2>
                <p className={styles.description}>
                    {waiting === 0
                        ? "Every open action item from this team's earlier retros has been reviewed."
                        : "Open action items from this team's earlier retros. Mark each one done, carry it over or drop it."}
                </p>
            </div>
            {membership.failure !== null && (
                <LoadFailure message={membership.failure.message} onRetry={membership.retry} />
            )}
            <ul className={styles.list}>
                {items.map(item => (
                    <PreviousActionRow
                        key={item.id}
                        item={item}
                        reviews={actions.review}
                        onReview={outcome => {
                            review.reset();
                            review.mutate({ itemId: item.id, outcome });
                        }}
                    />
                ))}
            </ul>
            {review.error !== null && (
                <Toast variant="error" onDismiss={review.reset}>
                    {review.error.message}
                </Toast>
            )}
        </section>
    );
}
