import { useState } from "react";
import { boardActions } from "../../domain/board/capabilities";
import { columnDropTarget } from "../../domain/board/dropTarget";
import { compareByPosition } from "../../domain/board/sort";
import { useClearColumn, useDeleteColumn, useMoveColumn, useUpdateColumn } from "../../hooks/mutations/boards";
import type { BoardSnapshot, ColumnView } from "../../types/api";
import { ConfirmDialog } from "../ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import { Modal } from "../Modal/Modal";
import { Toast } from "../Toast/Toast";
import styles from "./ColumnTools.module.css";
import { ColumnForm, type ColumnFields } from "./ColumnForm";

interface ColumnToolsProps {
    snapshot: BoardSnapshot;
    column: ColumnView;
}

type Confirming = "clear" | "delete" | null;

export function ColumnTools({ snapshot, column }: ColumnToolsProps) {
    const update = useUpdateColumn(snapshot.board_id);
    const move = useMoveColumn(snapshot.board_id);
    const clear = useClearColumn(snapshot.board_id);
    const removal = useDeleteColumn(snapshot.board_id);
    const [editing, setEditing] = useState(false);
    const [confirming, setConfirming] = useState<Confirming>(null);

    const columns = snapshot.columns.toSorted(compareByPosition);
    const index = columns.findIndex(candidate => candidate.id === column.id);
    const pending = confirming === "clear" ? clear : removal;

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

    function openConfirm(next: Exclude<Confirming, null>) {
        clear.reset();
        removal.reset();
        setConfirming(next);
    }

    function closeConfirm() {
        clear.reset();
        removal.reset();
        setConfirming(null);
    }

    function handleConfirm() {
        pending.mutate(
            { columnId: column.id },
            {
                onSuccess: () => {
                    setConfirming(null);
                },
            },
        );
    }

    const items: MenuItem[] = [
        {
            label: "Edit column",
            onSelect: () => {
                update.reset();
                setEditing(true);
            },
        },
    ];

    if (index > 0 && !move.isPending) {
        items.push({ label: "Move left", onSelect: () => moveTo(index - 1) });
    }

    if (index !== -1 && index < columns.length - 1 && !move.isPending) {
        items.push({ label: "Move right", onSelect: () => moveTo(index + 1) });
    }

    if (boardActions(snapshot).removeColumns) {
        items.push(
            { label: "Clear cards", onSelect: () => openConfirm("clear") },
            { label: "Delete column", tone: "danger", onSelect: () => openConfirm("delete") },
        );
    }

    const confirmBody = (
        <div className={styles.dialog}>
            {confirming === "clear" && (
                <p>
                    Every card in <bdi>{column.title}</bdi> is deleted for everyone, including cards that are still
                    hidden. The column stays.
                </p>
            )}
            {confirming === "delete" && (
                <p>
                    <bdi>{column.title}</bdi> and every card in it are deleted for everyone, including cards that are
                    still hidden.
                </p>
            )}
            {pending.error !== null && <ErrorBanner message={pending.error.message} takeFocus />}
        </div>
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
            <ConfirmDialog
                open={confirming !== null}
                title={confirming === "clear" ? `Clear the cards in ${column.title}?` : `Delete ${column.title}?`}
                body={confirmBody}
                confirmLabel={confirming === "clear" ? "Clear cards" : "Delete column"}
                destructive
                busy={pending.isPending}
                onConfirm={handleConfirm}
                onCancel={closeConfirm}
            />
            {move.error !== null && (
                <Toast variant="error" onDismiss={move.reset}>
                    {move.error.message}
                </Toast>
            )}
        </>
    );
}
