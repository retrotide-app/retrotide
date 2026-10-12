import { type ReactNode, useId } from "react";
import { type SummaryCard, votesLabel } from "../../../domain/board/summary";
import { MentionText } from "../mentions/MentionText";
import styles from "./BoardSummary.module.css";
import { SummaryActionList } from "./SummaryActionList";

interface SummaryCardItemProps {
    entry: SummaryCard;
    votes: number | null;
    children?: ReactNode;
}

export function SummaryCardItem({ entry, votes, children }: SummaryCardItemProps) {
    const bodyId = useId();
    const { card, comments, actionItems } = entry;

    return (
        <article className={styles.card} aria-labelledby={bodyId}>
            <p className={styles.body}>
                <MentionText id={bodyId} text={card.body} mentions={card.mentions ?? []} />
            </p>
            <p className={styles.meta}>
                {votes !== null && <span>{votesLabel(votes)}</span>}
                {card.author !== null && <bdi>{card.author.display_name}</bdi>}
                {card.hidden && <span>Only you can see this</span>}
            </p>
            {comments.length > 0 && (
                <ul className={styles.comments} aria-label="Comments">
                    {comments.map(comment => (
                        <li key={comment.id} className={styles.comment}>
                            <MentionText text={comment.body} mentions={comment.mentions ?? []} />
                            {comment.author !== null && (
                                <span className={styles.meta}>
                                    <bdi>{comment.author.display_name}</bdi>
                                </span>
                            )}
                        </li>
                    ))}
                </ul>
            )}
            {actionItems.length > 0 && <SummaryActionList label="Action items" items={actionItems} />}
            {children}
        </article>
    );
}
