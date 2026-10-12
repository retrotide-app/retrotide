import { useState } from "react";
import { sourceCard } from "../../../domain/board/actionItems";
import {
    actionItemActions,
    boardActions,
    cardInteractions,
    isArchived,
    NO_ACTION_ITEM_ACTIONS,
    NO_CARD_INTERACTIONS,
} from "../../../domain/board/capabilities";
import { facilitationRefusal } from "../../../domain/board/facilitation";
import { NO_MENTIONS } from "../../../domain/board/mentions";
import {
    nextPresented,
    presentationOrder,
    presentationProgress,
    presentedStack,
    previousPresented,
} from "../../../domain/board/presentation";
import { stackVotes } from "../../../domain/board/votes";
import { useBoardMembership } from "../../../hooks/board/useBoardMembership";
import type { CardFocus } from "../../../hooks/board/useCardFocus";
import { usePresentationKeys } from "../../../hooks/board/usePresentationKeys";
import { usePresentCard } from "../../../hooks/mutations/facilitation";
import type { BoardSnapshot } from "../../../types/api";
import { Button } from "../../Button/Button";
import { ErrorBanner } from "../../ErrorBanner/ErrorBanner";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { ActionItemRow } from "../actionItems/ActionItemRow";
import { MakeActionItemDialog } from "../actionItems/MakeActionItemDialog";
import { CardComments } from "../CardComments";
import { CardVotes } from "../CardVotes";
import { MentionText } from "../mentions/MentionText";
import { MergedCards } from "../MergedCards";
import styles from "./Presentation.module.css";

interface PresentationViewProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cardFocus: CardFocus;
    onLeave: () => void;
}

interface PresentedProps extends PresentationViewProps {
    cardId: string;
}

function voteLabel(total: number): string {
    return total === 1 ? "1 vote" : `${total} votes`;
}

function presentingLine(progressLabel: string): string {
    return progressLabel === "" ? "Presenting a card." : `Presenting ${progressLabel.toLocaleLowerCase()}.`;
}

export function PresentationView(props: PresentationViewProps) {
    const cardId = props.snapshot.spotlight.presenting_card_id;

    if (cardId === null) {
        return null;
    }

    return <Presented {...props} cardId={cardId} />;
}

function Presented({ snapshot, writable, cardFocus, onLeave, cardId }: PresentedProps) {
    const present = usePresentCard(snapshot.board_id);
    const [converting, setConverting] = useState(false);

    const { board } = snapshot;
    const inActions = board.stage === "actions";
    const membership = useBoardMembership(snapshot.board_id, writable && inActions && !isArchived(board));
    const steers = writable && boardActions(snapshot).spotlight;
    const order = presentationOrder(snapshot);
    const next = nextPresented(order, cardId);
    const previous = previousPresented(order, cardId);
    const progress = presentationProgress(order, cardId);
    const stack = presentedStack(snapshot, cardId);

    usePresentationKeys({
        presenting: true,
        steers,
        next,
        previous,
        onPresent: present.mutate,
        onLeave,
    });

    const progressLabel = progress === null ? "" : `Card ${progress.position} of ${progress.total}`;
    const itemActions = writable ? actionItemActions(snapshot, membership.member) : NO_ACTION_ITEM_ACTIONS;
    const people = itemActions.name ? membership.people : null;

    const controls = steers ? (
        <div className={styles.steering}>
            <Button
                type="button"
                disabled={previous === null || present.isPending}
                onClick={() => present.mutate({ cardId: previous })}
            >
                Previous
            </Button>
            <Button
                type="button"
                variant="primary"
                disabled={next === null || present.isPending}
                onClick={() => present.mutate({ cardId: next })}
            >
                Next
            </Button>
            <Button
                type="button"
                variant="danger"
                disabled={present.isPending}
                onClick={() => present.mutate({ cardId: null })}
            >
                End presentation
            </Button>
        </div>
    ) : (
        <div className={styles.steering}>
            <Button type="button" onClick={onLeave}>
                Leave presentation
            </Button>
        </div>
    );

    const announcement = stack === null ? "This card is no longer on the board." : presentingLine(progressLabel);

    if (stack === null) {
        return (
            <section className={styles.presentation} aria-label="Presentation">
                <p className="visually-hidden" role="status" aria-live="polite">
                    {announcement}
                </p>
                <div className={styles.header}>{controls}</div>
                <p className={styles.gone}>This card is no longer on the board</p>
                {present.error !== null && <ErrorBanner message={facilitationRefusal(present.error)} />}
            </section>
        );
    }

    const { card, merged, actionItems } = stack;
    const cards = [card, ...merged];
    const { total } = stackVotes(cards, snapshot.me);
    const interactions = writable ? cardInteractions(snapshot, card) : NO_CARD_INTERACTIONS;
    const makesActions = writable && inActions && !card.hidden && itemActions.add;

    return (
        <section className={styles.presentation} aria-label="Presentation">
            <p className="visually-hidden" role="status" aria-live="polite">
                {announcement}
            </p>
            <div className={styles.header}>
                {progressLabel !== "" && <p className={styles.progress}>{progressLabel}</p>}
                {controls}
            </div>
            {present.error !== null && <ErrorBanner message={facilitationRefusal(present.error)} />}
            <article className={styles.card} aria-label="Presented card">
                <p className={styles.body} dir="auto">
                    <MentionText text={card.body} mentions={card.mentions ?? NO_MENTIONS} />
                </p>
                {card.author !== null && (
                    <p className={styles.meta}>
                        <bdi>{card.author.display_name}</bdi>
                    </p>
                )}
                {board.stage === "vote" ? (
                    <CardVotes snapshot={snapshot} stack={cards} canVote={interactions.vote} />
                ) : (
                    total !== null && <p className={styles.meta}>{voteLabel(total)}</p>
                )}
                <MergedCards snapshot={snapshot} merged={merged} writable={writable} />
                {snapshot.settings.comments_enabled && (
                    <div className={styles.thread}>
                        <CardComments
                            snapshot={snapshot}
                            stack={cards}
                            canComment={interactions.comment}
                            writable={writable}
                        />
                    </div>
                )}
            </article>
            {(actionItems.length > 0 || makesActions) && (
                <div className={styles.actionItems}>
                    <h2 className={styles.subheading}>Action items</h2>
                    {membership.failure !== null && (
                        <LoadFailure message={membership.failure.message} onRetry={membership.retry} />
                    )}
                    {actionItems.length > 0 && (
                        <ul className={styles.list}>
                            {actionItems.map(item => (
                                <ActionItemRow
                                    key={item.id}
                                    boardId={snapshot.board_id}
                                    item={item}
                                    source={sourceCard(snapshot, item)}
                                    actions={itemActions}
                                    people={people}
                                    cardFocus={cardFocus}
                                />
                            ))}
                        </ul>
                    )}
                    {makesActions && (
                        <div>
                            <Button type="button" size="small" onClick={() => setConverting(true)}>
                                + Action
                            </Button>
                        </div>
                    )}
                </div>
            )}
            {converting && (
                <MakeActionItemDialog snapshot={snapshot} card={card} onClose={() => setConverting(false)} />
            )}
        </section>
    );
}
