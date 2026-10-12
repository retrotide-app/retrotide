import { useCallback, useState } from "react";
import type { BoardActions } from "../../domain/board/capabilities";
import { columnClipboardText } from "../../domain/board/clipboardText";
import { columnDropTarget } from "../../domain/board/dropTarget";
import { facilitationRefusal } from "../../domain/board/facilitation";
import { compareByPosition } from "../../domain/board/sort";
import { useMoveColumn, useUpdateColumn } from "../../hooks/mutations/boards";
import { useHideColumn, useResetColumnVotes, useRevealColumn, useShowColumn } from "../../hooks/mutations/facilitation";
import { useCopyText } from "../../hooks/useCopyText";
import type { BoardSnapshot, ColumnView } from "../../types/api";
import { ConfirmDialog } from "../ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import { Modal } from "../Modal/Modal";
import { Toast } from "../Toast/Toast";
import { ColumnForm, type ColumnFields } from "./ColumnForm";
import { ColumnVoteLimitDialog } from "./ColumnVoteLimitDialog";

interface ColumnMenuProps {
    snapshot: BoardSnapshot;
    column: ColumnView;
    actions: BoardActions;
}

export function ColumnMenu({ snapshot, column, actions }: ColumnMenuProps) {
    const boardId = snapshot.board_id;
    const update = useUpdateColumn(boardId);
    const move = useMoveColumn(boardId);
    const reveal = useRevealColumn(boardId);
    const hide = useHideColumn(boardId);
    const show = useShowColumn(boardId);
    const reset = useResetColumnVotes(boardId);
    const copy = useCopyText();
    const [editing, setEditing] = useState(false);
    const [limiting, setLimiting] = useState(false);
    const [confirmingReset, setConfirmingReset] = useState(false);

    const columns = snapshot.columns.toSorted(compareByPosition);
    const index = columns.findIndex(candidate => candidate.id === column.id);
    const { board } = snapshot;
    const refused = reveal.error ?? hide.error ?? show.error ?? move.error;

    function handleSave(fields: ColumnFields) {
        update.mutate(
            { columnId: column.id, ...fields },
            {
                onSuccess: () => {
                    setEditing(false);
                },
            },
        );
    }

    function moveTo(slot: number) {
        move.mutate({ columnId: column.id, ...columnDropTarget(columns, column.id, slot) });
    }

    function closeReset() {
        reset.reset();
        setConfirmingReset(false);
    }

    function handleReset() {
        reset.mutate(
            { columnId: column.id },
            {
                onSuccess: () => {
                    setConfirmingReset(false);
                },
            },
        );
    }

    const { reset: resetReveal } = reveal;
    const { reset: resetHide } = hide;
    const { reset: resetShow } = show;
    const { reset: resetMove } = move;
    const dismissRefusal = useCallback(() => {
        resetReveal();
        resetHide();
        resetShow();
        resetMove();
    }, [resetReveal, resetHide, resetShow, resetMove]);

    const items: MenuItem[] = [];

    if (actions.manageColumns) {
        items.push({
            label: "Edit column",
            onSelect: () => {
                update.reset();
                setEditing(true);
            },
        });

        if (index > 0 && !move.isPending) {
            items.push({ label: "Move left", onSelect: () => moveTo(index - 1) });
        }

        if (index !== -1 && index < columns.length - 1 && !move.isPending) {
            items.push({ label: "Move right", onSelect: () => moveTo(index + 1) });
        }
    }

    if (actions.spotlight) {
        if (board.stage === "write" && board.cards_hidden && !column.cards_revealed && !column.hidden) {
            items.push({ label: "Reveal cards", onSelect: () => reveal.mutate({ columnId: column.id }) });
        }

        if (column.hidden) {
            items.push({ label: "Show to everyone", onSelect: () => show.mutate({ columnId: column.id }) });
        } else {
            items.push({ label: "Hide from participants", onSelect: () => hide.mutate({ columnId: column.id }) });
        }

        items.push({ label: "Vote limit…", onSelect: () => setLimiting(true) });
    }

    items.push({ label: "Copy column", onSelect: () => copy.copy(columnClipboardText(snapshot, column.id)) });

    if (actions.spotlight && !column.hidden) {
        items.push({
            label: "Reset votes",
            tone: "danger",
            onSelect: () => {
                reset.reset();
                setConfirmingReset(true);
            },
        });
    }

    const resetBody = (
        <>
            <p>
                Every vote on the cards in <bdi>{column.title}</bdi> is taken back for everyone and returned to the
                person who cast it.
            </p>
            {reset.error !== null && <ErrorBanner message={facilitationRefusal(reset.error)} takeFocus />}
        </>
    );

    return (
        <>
            <Menu label={`Options for ${column.title}`} items={items} iconOnly size="small" />
            <Modal isOpen={editing} onClose={() => setEditing(false)} title={`Edit ${column.title}`}>
                <ColumnForm
                    label={`Edit column ${column.title}`}
                    initial={{ title: column.title, description: column.description, colour: column.colour }}
                    submitLabel="Save column"
                    pending={update.isPending}
                    error={update.error}
                    onSubmit={handleSave}
                    onCancel={() => setEditing(false)}
                />
            </Modal>
            {limiting && <ColumnVoteLimitDialog boardId={boardId} column={column} onClose={() => setLimiting(false)} />}
            <ConfirmDialog
                open={confirmingReset}
                title={`Reset the votes in ${column.title}?`}
                body={resetBody}
                confirmLabel="Reset votes"
                destructive
                busy={reset.isPending}
                onConfirm={handleReset}
                onCancel={closeReset}
            />
            {refused !== null && (
                <Toast variant="error" onDismiss={dismissRefusal}>
                    {facilitationRefusal(refused)}
                </Toast>
            )}
            {copy.copied && (
                <Toast variant="success" onDismiss={copy.reset}>
                    Column copied.
                </Toast>
            )}
            {copy.error !== null && (
                <Toast variant="error" onDismiss={copy.reset}>
                    {copy.error.message}
                </Toast>
            )}
        </>
    );
}
