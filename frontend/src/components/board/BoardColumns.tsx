import { PointerActivationConstraints } from "@dnd-kit/dom";
import { DragDropProvider, type DragEndEvent, type DragMoveEvent, KeyboardSensor, PointerSensor } from "@dnd-kit/react";
import { useId, useState } from "react";
import { boardActions, type BoardActions, boardInteractions, NO_INTERACTIONS } from "../../domain/board/capabilities";
import { type CardFilter, keepsBoardOrder, UNFILTERED } from "../../domain/board/cardFilter";
import {
    type BoardMove,
    type DropPreview,
    dropPreview,
    type DropSide,
    readDragData,
    resolveDrop,
    sameDropPreview,
} from "../../domain/board/drop";
import { type BoardColumnModel, boardColumns } from "../../domain/board/selectors";
import type { CardFocus } from "../../hooks/board/useCardFocus";
import { useMergeCard, useMoveCard } from "../../hooks/mutations/boards";
import type { BoardSnapshot } from "../../types/api";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { AddColumnForm } from "./AddColumnForm";
import { BoardColumn } from "./BoardColumn";
import styles from "./BoardColumns.module.css";
import colours from "./boardColours.module.css";

interface BoardColumnsProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cardFocus: CardFocus;
    filter?: CardFilter;
}

type DragOperation = DragEndEvent["operation"];

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

const POINTER_DISTANCE_PX = 5;

const TOUCH_PRESS_MS = 250;

const TOUCH_TOLERANCE_PX = 5;

const BOARD_SENSORS = [
    PointerSensor.configure({
        activationConstraints: event =>
            event.pointerType === "touch"
                ? [new PointerActivationConstraints.Delay({ value: TOUCH_PRESS_MS, tolerance: TOUCH_TOLERANCE_PX })]
                : [new PointerActivationConstraints.Distance({ value: POINTER_DISTANCE_PX })],
    }),
    KeyboardSensor,
];

export function BoardColumns({ snapshot, writable, cardFocus, filter = UNFILTERED }: BoardColumnsProps) {
    const { mutate: moveCard, error: moveError } = useMoveCard(snapshot.board_id);
    const { mutate: mergeCard, error: mergeError } = useMergeCard(snapshot.board_id);

    const actions = writable ? boardActions(snapshot) : NO_BOARD_ACTIONS;
    const interactions = writable ? boardInteractions(snapshot) : NO_INTERACTIONS;
    const rules = {
        reorder: actions.moveCards && keepsBoardOrder(filter, snapshot.board),
        merge: interactions.mergeCards,
    };
    const columns = boardColumns(snapshot, filter);
    const failure = moveError ?? mergeError;
    const [preview, setPreview] = useState<DropPreview | null>(null);
    const stripId = useId();
    const focusedId = snapshot.spotlight.focused_column_id;
    const focused = columns.find(model => model.column.id === focusedId) ?? null;
    const addsColumns = actions.manageColumns && focused === null;

    function moveFor(operation: DragOperation): BoardMove | null {
        const dragged = readDragData(operation.source?.data);
        const onto = readDragData(operation.target?.data);

        if (dragged === null || onto === null) {
            return null;
        }

        const centre = operation.target?.shape?.center;
        const side: DropSide = centre !== undefined && operation.position.current.y > centre.y ? "after" : "before";

        return resolveDrop(snapshot, dragged, onto, rules, side);
    }

    function handleDragMove({ operation }: DragMoveEvent) {
        const next = dropPreview(snapshot, moveFor(operation));

        setPreview(current => (sameDropPreview(current, next) ? current : next));
    }

    function handleDragEnd({ operation, canceled }: DragEndEvent) {
        setPreview(null);

        if (canceled) {
            return;
        }

        const move = moveFor(operation);

        if (move?.kind === "merge") {
            mergeCard({ cardId: move.cardId, targetCardId: move.targetCardId });
            return;
        }

        if (move?.kind !== "card") {
            return;
        }

        moveCard({ cardId: move.cardId, ...move.drop });
    }

    function columnOf(model: BoardColumnModel) {
        return (
            <BoardColumn
                key={model.column.id}
                snapshot={snapshot}
                model={model}
                actions={actions}
                writable={writable}
                cardFocus={cardFocus}
                reorder={rules.reorder}
                merge={rules.merge}
                preview={preview}
            />
        );
    }

    return (
        <div className={styles.board}>
            {failure !== null && <ErrorBanner message={failure.message} takeFocus />}
            <p className="visually-hidden" role="status">
                {focused === null ? "" : `Focused on ${focused.column.title}`}
            </p>
            <DragDropProvider
                sensors={BOARD_SENSORS}
                onDragMove={handleDragMove}
                onDragOver={handleDragMove}
                onDragEnd={handleDragEnd}
            >
                <div className={focused === null ? styles.columns : `${styles.columns} ${styles.focusing}`}>
                    {columns.map(model => {
                        if (focused === null) {
                            return columnOf(model);
                        }

                        if (model.column.id === focused.column.id) {
                            return (
                                <div key={model.column.id} className={styles.wide}>
                                    {columnOf(model)}
                                </div>
                            );
                        }

                        return (
                            <section
                                key={model.column.id}
                                aria-labelledby={`${stripId}-${model.column.id}`}
                                className={`${styles.strip} ${colours[model.column.colour]}`}
                            >
                                <h2 id={`${stripId}-${model.column.id}`} className={styles.stripTitle} dir="auto">
                                    {model.column.title}
                                </h2>
                            </section>
                        );
                    })}
                    {columns.length === 0 && <p>This board has no columns yet.</p>}
                    {addsColumns && (
                        <div className={styles.addColumn}>
                            <AddColumnForm boardId={snapshot.board_id} />
                        </div>
                    )}
                </div>
            </DragDropProvider>
        </div>
    );
}
