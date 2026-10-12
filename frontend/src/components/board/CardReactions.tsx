import { cardPolishActions } from "../../domain/board/capabilities";
import { REACTIONS, reactionOption } from "../../domain/board/reactions";
import { useAddReaction, useRemoveReaction } from "../../hooks/mutations/cardPolish";
import type { BoardSnapshot, CardView, ReactionCount, ReactionKey } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import styles from "./CardReactions.module.css";

interface CardReactionsProps {
    snapshot: BoardSnapshot;
    card: CardView;
    writable: boolean;
}

const NO_COUNTS: readonly ReactionCount[] = [];

const NO_REACTIONS: readonly ReactionKey[] = [];

function countLabel(label: string, count: number, mine: boolean): string {
    const people = count === 1 ? "1 reaction" : `${count} reactions`;

    return mine ? `${label}: ${people}, yours included` : `${label}: ${people}`;
}

export function CardReactions({ snapshot, card, writable }: CardReactionsProps) {
    const add = useAddReaction(snapshot.board_id);
    const remove = useRemoveReaction(snapshot.board_id);

    const reacts = writable && cardPolishActions(snapshot, card).react;
    const counts = card.reactions ?? NO_COUNTS;
    const mine = card.my_reactions ?? NO_REACTIONS;
    const failure = add.error ?? remove.error;

    if (!reacts && counts.length === 0) {
        return null;
    }

    function toggle(reaction: ReactionKey) {
        add.reset();
        remove.reset();

        if (mine.includes(reaction)) {
            remove.mutate({ cardId: card.id, reaction });
            return;
        }

        add.mutate({ cardId: card.id, reaction });
    }

    const choices: MenuItem[] = REACTIONS.map(option => ({
        label: `${option.emoji} ${option.label}`,
        onSelect: () => toggle(option.key),
    }));

    return (
        <div className={styles.reactions}>
            {failure !== null && <ErrorBanner message={failure.message} />}
            <div role="group" aria-label="Reactions" className={styles.chips}>
                {counts.map(count => {
                    const option = reactionOption(count.reaction);
                    const pressed = mine.includes(count.reaction);

                    return (
                        <Button
                            key={count.reaction}
                            type="button"
                            size="small"
                            variant={pressed ? "primary" : "secondary"}
                            aria-pressed={pressed}
                            aria-label={countLabel(option.label, count.count, pressed)}
                            disabled={!reacts}
                            onClick={() => toggle(count.reaction)}
                        >
                            <span aria-hidden="true">{option.emoji}</span>
                            <span aria-hidden="true">{count.count}</span>
                        </Button>
                    );
                })}
                {reacts && <Menu label="Add reaction" items={choices} size="small" />}
            </div>
        </div>
    );
}
