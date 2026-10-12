import { columnVotesLeft, heldVoteCard, stackVotes, votesLeftOnCard } from "../../domain/board/votes";
import { useCastVote, useRemoveVote } from "../../hooks/mutations/boards";
import type { BoardSnapshot, CardView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./BoardCard.module.css";

interface CardVotesProps {
    snapshot: BoardSnapshot;
    stack: readonly CardView[];
    canVote: boolean;
}

function totalLabel(total: number | null): string {
    if (total === null) {
        return "Totals hidden";
    }

    return total === 1 ? "1 vote" : `${total} votes`;
}

export function CardVotes({ snapshot, stack, canVote }: CardVotesProps) {
    const cast = useCastVote(snapshot.board_id);
    const remove = useRemoveVote(snapshot.board_id);

    const [top] = stack;
    const { total, mine } = stackVotes(stack, snapshot.me);
    const heldId = heldVoteCard(stack, snapshot.me);
    const left = votesLeftOnCard(snapshot, top);
    const columnSpent = columnVotesLeft(snapshot, top.column_id) === 0;
    const voteLabel = columnSpent ? "No votes left in this column" : "Vote for this card";
    const failure = cast.error ?? remove.error;

    return (
        <div className={styles.votes}>
            {failure !== null && <ErrorBanner message={failure.message} />}
            <div role="group" aria-label="Votes" className={styles.voteControls}>
                {canVote && (
                    <Button
                        type="button"
                        size="small"
                        variant="ghost"
                        aria-label="Take back a vote"
                        disabled={heldId === null}
                        onClick={() => {
                            if (heldId !== null) {
                                remove.mutate({ cardId: heldId });
                            }
                        }}
                    >
                        <span aria-hidden="true">−</span>
                    </Button>
                )}
                <span className={styles.voteCount}>
                    {totalLabel(total)}
                    {mine > 0 && <span className={styles.mine}>{` · ${mine} yours`}</span>}
                </span>
                {canVote && (
                    <Button
                        type="button"
                        size="small"
                        variant="ghost"
                        aria-label={voteLabel}
                        title={columnSpent ? voteLabel : undefined}
                        disabled={left === 0 || columnSpent}
                        onClick={() => cast.mutate({ cardId: top.id })}
                    >
                        <span aria-hidden="true">+</span>
                    </Button>
                )}
            </div>
        </div>
    );
}
