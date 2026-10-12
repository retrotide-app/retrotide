import { useContext, useEffect, useId, useRef, useState } from "react";
import { actionCountLabel, cardActionCount } from "../../domain/board/actionItems";
import {
    actionItemActions,
    boardActions,
    cardActions,
    type CardActions,
    cardInteractions,
    cardPolishActions,
    type CardPolishActions,
    columnCardsHidden,
    columnHidden,
    NO_CARD_INTERACTIONS,
} from "../../domain/board/capabilities";
import { cardClipboardText } from "../../domain/board/clipboardText";
import { facilitationRefusal } from "../../domain/board/facilitation";
import { mentionsChanged, NO_MENTIONS } from "../../domain/board/mentions";
import { stackVotes } from "../../domain/board/votes";
import { useArrival } from "../../hooks/board/useArrivals";
import type { CardFocus } from "../../hooks/board/useCardFocus";
import { CardDeleteUndoContext } from "../../hooks/board/useDeferredCardDelete";
import { useDeleteCard, useEditCard, useRecolourCard } from "../../hooks/mutations/boards";
import { usePinCard } from "../../hooks/mutations/cardPolish";
import { useHighlightCard, usePresentCard } from "../../hooks/mutations/facilitation";
import { useCopyText } from "../../hooks/useCopyText";
import type { BoardSnapshot, CardView } from "../../types/api";
import { formatDate } from "../../utils/time";
import { Button } from "../Button/Button";
import { ConfirmDialog } from "../ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import { Modal } from "../Modal/Modal";
import { StatusTag } from "../StatusTag/StatusTag";
import { Toast } from "../Toast/Toast";
import { MakeActionItemDialog } from "./actionItems/MakeActionItemDialog";
import styles from "./BoardCard.module.css";
import colours from "./boardColours.module.css";
import { CardComments } from "./CardComments";
import { CardForm, type CardFields } from "./CardForm";
import { CardMergeForm } from "./CardMergeForm";
import { CardMoveForm } from "./CardMoveForm";
import { CardReactions } from "./CardReactions";
import { CardVotes } from "./CardVotes";
import { CardImage } from "./images/CardImage";
import { CardImageDialog } from "./images/CardImageDialog";
import { MentionText } from "./mentions/MentionText";
import { MergedCards } from "./MergedCards";

interface BoardCardProps {
    snapshot: BoardSnapshot;
    card: CardView;
    merged?: readonly CardView[];
    writable: boolean;
    cardFocus: CardFocus;
}

type Dialog = "edit" | "move" | "merge" | "delete" | "convert" | "image" | null;

const NO_ACTIONS: CardActions = { edit: false, recolour: false, remove: false, move: false };

const NO_POLISH: CardPolishActions = { react: false, pin: false, setImage: false };

const NO_MERGED: readonly CardView[] = [];

const REVEAL_MS = 1600;

export function BoardCard({ snapshot, card, merged = NO_MERGED, writable, cardFocus }: BoardCardProps) {
    const bodyId = useId();
    const cardRef = useRef<HTMLElement>(null);
    const edit = useEditCard(snapshot.board_id);
    const recolour = useRecolourCard(snapshot.board_id);
    const removal = useDeleteCard(snapshot.board_id);
    const highlight = useHighlightCard(snapshot.board_id);
    const present = usePresentCard(snapshot.board_id);
    const pin = usePinCard(snapshot.board_id);
    const copy = useCopyText();
    const deferred = useContext(CardDeleteUndoContext);
    const [dialog, setDialog] = useState<Dialog>(null);
    const [revealed, setRevealed] = useState(false);

    const actions = writable ? cardActions(snapshot, card) : NO_ACTIONS;
    const interactions = writable ? cardInteractions(snapshot, card) : NO_CARD_INTERACTIONS;
    const polish = writable ? cardPolishActions(snapshot, card) : NO_POLISH;
    const mentions = card.mentions ?? NO_MENTIONS;
    const stack = [card, ...merged];
    const { total, mine } = stackVotes(stack, snapshot.me);
    const votingNow = snapshot.board.stage === "vote" && snapshot.settings.voting_open;
    const showsVotes = votingNow || mine > 0 || (total !== null && total > 0);
    const showsReactions = polish.react || (card.reactions?.length ?? 0) > 0;
    const actionCount = cardActionCount(snapshot, stack);
    const makesActions = writable && !card.hidden && actionItemActions(snapshot, false).add;
    const showsFooter =
        showsVotes || showsReactions || snapshot.settings.comments_enabled || makesActions || actionCount > 0;
    const shownDate = snapshot.settings.show_card_dates && card.created_at !== undefined ? card.created_at : null;
    const colourClass = card.colour === null ? "" : colours[card.colour];
    const arriving = useArrival(card.id);
    const highlightedId = snapshot.spotlight.highlighted_card_id;
    const highlighted = highlightedId !== null && stack.some(held => held.id === highlightedId);
    const spotlights =
        writable &&
        boardActions(snapshot).spotlight &&
        !card.hidden &&
        !columnCardsHidden(snapshot, card.column_id) &&
        !columnHidden(snapshot, card.column_id);
    const spotlightFailure = highlight.error ?? present.error;
    const classes = [
        styles.card,
        colourClass,
        arriving ? styles.arriving : "",
        revealed ? styles.revealed : "",
        highlighted ? styles.highlighted : "",
    ]
        .filter(Boolean)
        .join(" ");

    useEffect(() => {
        if (cardFocus.claim(card.id)) {
            cardRef.current?.focus();
        }
    }, [cardFocus, card.id]);

    useEffect(
        () =>
            cardFocus.watch(card.id, () => {
                cardRef.current?.scrollIntoView({ block: "center" });
                cardRef.current?.focus({ preventScroll: true });
                setRevealed(true);
            }),
        [cardFocus, card.id],
    );

    useEffect(() => {
        if (!revealed) {
            return;
        }

        const timer = window.setTimeout(() => setRevealed(false), REVEAL_MS);

        return () => window.clearTimeout(timer);
    }, [revealed]);

    if (deferred?.hidden.has(card.id) === true) {
        return null;
    }

    function saveColour(fields: CardFields) {
        if (fields.colour === card.colour) {
            setDialog(null);
            return;
        }

        recolour.mutate({ cardId: card.id, colour: fields.colour }, { onSuccess: () => setDialog(null) });
    }

    function handleSave(fields: CardFields) {
        if (fields.body === card.body && !mentionsChanged(mentions, fields.mentions)) {
            saveColour(fields);
            return;
        }

        edit.mutate(
            { cardId: card.id, body: fields.body, mentions: fields.mentions },
            { onSuccess: () => saveColour(fields) },
        );
    }

    function handleDelete() {
        if (deferred === null) {
            removal.mutate({ cardId: card.id }, { onSuccess: () => setDialog(null) });
            return;
        }

        deferred.schedule(card.id);
        setDialog(null);
    }

    const items: MenuItem[] = [];

    if (actions.edit || actions.recolour) {
        items.push({
            label: "Edit card",
            onSelect: () => {
                edit.reset();
                recolour.reset();
                setDialog("edit");
            },
        });
    }

    if (actions.move) {
        items.push({ label: "Move card", onSelect: () => setDialog("move") });
    }

    if (interactions.merge) {
        items.push({ label: "Merge into another card", onSelect: () => setDialog("merge") });
    }

    if (spotlights) {
        items.push({
            label: highlighted ? "Remove highlight" : "Highlight for everyone",
            onSelect: () => {
                present.reset();
                highlight.mutate({ cardId: highlighted ? null : card.id });
            },
        });
        items.push({
            label: "Present from this card",
            onSelect: () => {
                highlight.reset();
                present.mutate({ cardId: card.id });
            },
        });
    }

    if (polish.pin) {
        items.push({
            label: card.pinned === true ? "Unpin" : "Pin to top",
            onSelect: () => {
                pin.reset();
                pin.mutate({ cardId: card.id, pinned: card.pinned !== true });
            },
        });
    }

    if (polish.setImage) {
        items.push({
            label: card.image_path === undefined ? "Add image" : "Change image",
            onSelect: () => setDialog("image"),
        });
    }

    items.push({ label: "Copy text", onSelect: () => copy.copy(cardClipboardText(snapshot, card)) });

    if (actions.remove) {
        items.push({
            label: "Delete card",
            tone: "danger",
            onSelect: () => {
                removal.reset();
                setDialog("delete");
            },
        });
    }

    const deleted =
        merged.length > 0
            ? "The card is deleted for everyone on the board, with its votes and comments. The cards merged into it stay on the board."
            : "The card is deleted for everyone on the board, with its votes and comments.";
    const undoNote = deferred === null ? "This cannot be undone." : "You can undo it for a few seconds afterwards.";

    const deleteBody = (
        <div className={styles.dialog}>
            <p>{`${deleted} ${undoNote}`}</p>
            {removal.error !== null && <ErrorBanner message={removal.error.message} takeFocus />}
        </div>
    );

    return (
        <article ref={cardRef} tabIndex={-1} className={classes} aria-labelledby={bodyId}>
            <div className={styles.top}>
                <p id={bodyId} className={styles.body} dir="auto">
                    <MentionText text={card.body} mentions={mentions} />
                </p>
                {card.pinned === true && <StatusTag tone="neutral">Pinned</StatusTag>}
                {highlighted && <StatusTag tone="warning">Highlighted</StatusTag>}
                <Menu label="Card options" items={items} iconOnly size="small" />
            </div>
            {card.image_path !== undefined && <CardImage path={card.image_path} alt="Image on the card" />}
            {(card.author !== null || card.hidden || shownDate !== null) && (
                <p className={styles.meta}>
                    {card.author !== null && <bdi>{card.author.display_name}</bdi>}
                    {card.author !== null && shownDate !== null && " · "}
                    {shownDate !== null && <time dateTime={shownDate}>{formatDate(shownDate)}</time>}
                    {(card.author !== null || shownDate !== null) && card.hidden && " · "}
                    {card.hidden && "Only you can see this"}
                </p>
            )}
            {merged.length > 0 && <MergedCards snapshot={snapshot} merged={merged} writable={writable} />}
            {showsFooter && (
                <div className={styles.footer}>
                    {showsVotes && <CardVotes snapshot={snapshot} stack={stack} canVote={interactions.vote} />}
                    {(makesActions || actionCount > 0) && (
                        <div className={styles.cardActions}>
                            {actionCount > 0 && <StatusTag tone="neutral">{actionCountLabel(actionCount)}</StatusTag>}
                            {makesActions && (
                                <Button type="button" size="small" onClick={() => setDialog("convert")}>
                                    + Action
                                </Button>
                            )}
                        </div>
                    )}
                    {showsReactions && <CardReactions snapshot={snapshot} card={card} writable={writable} />}
                    {snapshot.settings.comments_enabled && (
                        <CardComments
                            snapshot={snapshot}
                            stack={stack}
                            canComment={interactions.comment}
                            writable={writable}
                        />
                    )}
                </div>
            )}
            <Modal isOpen={dialog === "edit"} onClose={() => setDialog(null)} title="Edit card">
                <CardForm
                    boardId={snapshot.board_id}
                    label="Edit card"
                    initial={{ body: card.body, colour: card.colour, mentions }}
                    submitLabel="Save card"
                    pending={edit.isPending || recolour.isPending}
                    error={edit.error ?? recolour.error}
                    onSubmit={handleSave}
                    onCancel={() => setDialog(null)}
                />
            </Modal>
            <Modal isOpen={dialog === "move"} onClose={() => setDialog(null)} title="Move card">
                <CardMoveForm snapshot={snapshot} card={card} cardFocus={cardFocus} />
            </Modal>
            <Modal isOpen={dialog === "merge"} onClose={() => setDialog(null)} title="Merge card">
                <CardMergeForm snapshot={snapshot} card={card} onMerged={() => setDialog(null)} />
            </Modal>
            {dialog === "convert" && (
                <MakeActionItemDialog snapshot={snapshot} card={card} onClose={() => setDialog(null)} />
            )}
            {dialog === "image" && <CardImageDialog snapshot={snapshot} card={card} onClose={() => setDialog(null)} />}
            {spotlightFailure !== null && (
                <Toast variant="error" onDismiss={highlight.error === null ? present.reset : highlight.reset}>
                    {facilitationRefusal(spotlightFailure)}
                </Toast>
            )}
            {pin.error !== null && (
                <Toast variant="error" onDismiss={pin.reset}>
                    {pin.error.message}
                </Toast>
            )}
            {copy.copied && (
                <Toast variant="success" onDismiss={copy.reset}>
                    Card text copied.
                </Toast>
            )}
            {copy.error !== null && (
                <Toast variant="error" onDismiss={copy.reset}>
                    {copy.error.message}
                </Toast>
            )}
            <ConfirmDialog
                open={dialog === "delete"}
                title="Delete this card?"
                body={deleteBody}
                confirmLabel="Delete card"
                destructive
                busy={removal.isPending}
                onConfirm={handleDelete}
                onCancel={() => setDialog(null)}
            />
        </article>
    );
}
