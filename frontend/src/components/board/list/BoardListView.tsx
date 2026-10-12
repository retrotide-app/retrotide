import { useId } from "react";
import { boardActions, type BoardActions } from "../../../domain/board/capabilities";
import { type CardFilter, UNFILTERED } from "../../../domain/board/cardFilter";
import { type BoardColumnModel, boardColumns } from "../../../domain/board/selectors";
import type { CardFocus } from "../../../hooks/board/useCardFocus";
import type { BoardSnapshot } from "../../../types/api";
import { AddCardForm } from "../AddCardForm";
import { BoardCard } from "../BoardCard";
import colours from "../boardColours.module.css";
import { HiddenCards } from "../HiddenCards";
import styles from "./ListView.module.css";

interface BoardListViewProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cardFocus: CardFocus;
    filter?: CardFilter;
}

interface ListColumnProps {
    snapshot: BoardSnapshot;
    model: BoardColumnModel;
    actions: BoardActions;
    writable: boolean;
    cardFocus: CardFocus;
}

const NO_BOARD_ACTIONS: BoardActions = {
    addCards: false,
    moveCards: false,
    manageColumns: false,
    removeAnyCard: false,
    describeBoard: false,
    removeColumns: false,
    controlTimer: false,
    spotlight: false,
};

function cardCount(total: number): string {
    return total === 1 ? "1 card" : `${total} cards`;
}

function ListColumn({ snapshot, model, actions, writable, cardFocus }: ListColumnProps) {
    const titleId = useId();
    const { column, stacks, placeholders, total, filtered } = model;

    return (
        <section aria-labelledby={titleId} className={`${styles.section} ${colours[column.colour]}`}>
            <header className={styles.header}>
                <h2 id={titleId} className={styles.title} dir="auto">
                    {column.title}
                </h2>
                <span className={styles.count}>{cardCount(total)}</span>
                {column.description !== "" && (
                    <p className={styles.description} dir="auto">
                        {column.description}
                    </p>
                )}
            </header>
            {actions.addCards && !column.hidden && <AddCardForm boardId={snapshot.board_id} column={column} />}
            {stacks.length > 0 && (
                <ol className={styles.cards} aria-label={`Cards in ${column.title}`}>
                    {stacks.map(({ card, merged }) => (
                        <li key={card.id}>
                            <BoardCard
                                snapshot={snapshot}
                                card={card}
                                merged={merged}
                                writable={writable}
                                cardFocus={cardFocus}
                            />
                        </li>
                    ))}
                </ol>
            )}
            <HiddenCards placeholders={placeholders} />
            {total === 0 && <p className={styles.count}>{filtered ? "No matching cards." : "No cards yet."}</p>}
        </section>
    );
}

export function BoardListView({ snapshot, writable, cardFocus, filter = UNFILTERED }: BoardListViewProps) {
    const actions = writable ? boardActions(snapshot) : NO_BOARD_ACTIONS;
    const columns = boardColumns(snapshot, filter);

    return (
        <div className={styles.list}>
            {columns.map(model => (
                <ListColumn
                    key={model.column.id}
                    snapshot={snapshot}
                    model={model}
                    actions={actions}
                    writable={writable}
                    cardFocus={cardFocus}
                />
            ))}
            {columns.length === 0 && <p>This board has no columns yet.</p>}
        </div>
    );
}
