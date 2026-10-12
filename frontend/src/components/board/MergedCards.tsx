import { cardInteractions } from "../../domain/board/capabilities";
import { NO_MENTIONS } from "../../domain/board/mentions";
import { useUnmergeCard } from "../../hooks/mutations/boards";
import type { BoardSnapshot, CardView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./BoardCard.module.css";
import { MentionText } from "./mentions/MentionText";

interface MergedCardsProps {
    snapshot: BoardSnapshot;
    merged: readonly CardView[];
    writable: boolean;
}

export function MergedCards({ snapshot, merged, writable }: MergedCardsProps) {
    const unmerge = useUnmergeCard(snapshot.board_id);

    if (merged.length === 0) {
        return null;
    }

    return (
        <div className={styles.merged}>
            {unmerge.error !== null && <ErrorBanner message={unmerge.error.message} />}
            <ul className={styles.mergedList} aria-label="Merged cards">
                {merged.map(card => (
                    <li key={card.id} className={styles.mergedCard}>
                        <div className={styles.top}>
                            <p className={styles.mergedBody} dir="auto">
                                <MentionText text={card.body} mentions={card.mentions ?? NO_MENTIONS} />
                            </p>
                            {writable && cardInteractions(snapshot, card).unmerge && (
                                <Button
                                    type="button"
                                    size="small"
                                    variant="ghost"
                                    aria-label={`Unmerge "${card.body}"`}
                                    disabled={unmerge.isPending}
                                    onClick={() => unmerge.mutate({ cardId: card.id })}
                                >
                                    Unmerge
                                </Button>
                            )}
                        </div>
                        {card.author !== null && (
                            <p className={styles.meta}>
                                <bdi>{card.author.display_name}</bdi>
                            </p>
                        )}
                    </li>
                ))}
            </ul>
        </div>
    );
}
