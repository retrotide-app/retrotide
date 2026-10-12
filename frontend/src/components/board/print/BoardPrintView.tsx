import { type Ref, useId } from "react";
import { type PrintCard, printColumns } from "../../../domain/board/print";
import { hiddenCardsLabel, votesLabel } from "../../../domain/board/summary";
import type { BoardSnapshot } from "../../../types/api";
import colours from "../boardColours.module.css";
import { CardImage } from "../images/CardImage";
import styles from "./BoardPrint.module.css";

interface BoardPrintViewProps {
    snapshot: BoardSnapshot;
    ref?: Ref<HTMLElement>;
}

function PrintedCard({ card }: { card: PrintCard }) {
    const hasMeta = card.pinned || card.votes !== null || card.reactions.length > 0;

    return (
        <li className={styles.card}>
            {card.imagePath !== null && <CardImage path={card.imagePath} alt={`Image on the card ${card.body}`} />}
            <p className={styles.body} dir="auto">
                {card.body}
            </p>
            {card.merged.length > 0 && (
                <ul className={styles.merged} aria-label="Merged cards">
                    {card.merged.map(merged => (
                        <li key={merged.id} dir="auto">
                            {merged.body}
                        </li>
                    ))}
                </ul>
            )}
            {hasMeta && (
                <div className={styles.meta}>
                    {card.pinned && <span className={styles.pinned}>Pinned</span>}
                    {card.votes !== null && <span>{votesLabel(card.votes)}</span>}
                    {card.reactions.length > 0 && (
                        <ul className={styles.reactions} aria-label="Reactions">
                            {card.reactions.map(reaction => (
                                <li key={reaction.key} aria-label={`${reaction.label}: ${reaction.count}`}>
                                    <span aria-hidden="true">{reaction.emoji}</span> {reaction.count}
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            )}
        </li>
    );
}

export function BoardPrintView({ snapshot, ref }: BoardPrintViewProps) {
    const titleId = useId();
    const columns = printColumns(snapshot);

    return (
        <article ref={ref} className={styles.sheet} aria-labelledby={titleId}>
            <h1 id={titleId} className={styles.title} dir="auto">
                {snapshot.board.title}
            </h1>
            <div className={styles.columns}>
                {columns.map(({ column, cards, hidden }) => (
                    <section
                        key={column.id}
                        className={`${styles.column} ${colours[column.colour]}`}
                        aria-label={column.title}
                    >
                        <h2 className={styles.columnTitle} dir="auto">
                            {column.title}
                        </h2>
                        {cards.length > 0 && (
                            <ul className={styles.cards}>
                                {cards.map(card => (
                                    <PrintedCard key={card.id} card={card} />
                                ))}
                            </ul>
                        )}
                        {hidden > 0 && <p className={styles.hidden}>{hiddenCardsLabel(hidden)}</p>}
                    </section>
                ))}
            </div>
        </article>
    );
}
