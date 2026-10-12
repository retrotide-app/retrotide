import { useId, useState } from "react";
import {
    ACTION_ITEM_STATUS_LABELS,
    formatDueDate,
    nextStatus,
    revealedCardId,
} from "../../../domain/board/actionItems";
import { type ActionItemActions, canRemoveActionItem } from "../../../domain/board/capabilities";
import type { CardFocus } from "../../../hooks/board/useCardFocus";
import { useDeleteActionItem, useEditActionItem, useMarkActionItem } from "../../../hooks/mutations/actionItems";
import type { ActionItemStatus, ActionItemView, CardView, PersonView } from "../../../types/api";
import { Button } from "../../Button/Button";
import { ConfirmDialog } from "../../ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../../Menu/Menu";
import { Modal } from "../../Modal/Modal";
import { StatusTag, type StatusTone } from "../../StatusTag/StatusTag";
import { Toast } from "../../Toast/Toast";
import { MentionText } from "../mentions/MentionText";
import { ActionItemForm, type ActionItemSubmission } from "./ActionItemForm";
import styles from "./ActionItems.module.css";

interface ActionItemRowProps {
    boardId: string;
    item: ActionItemView;
    source: CardView | null;
    actions: ActionItemActions;
    people: readonly PersonView[] | null;
    cardFocus: CardFocus;
}

type Dialog = "edit" | "delete" | null;

const STATUS_TONES: Readonly<Record<ActionItemStatus, StatusTone>> = {
    open: "warning",
    done: "success",
    dropped: "neutral",
};

export function ActionItemRow({ boardId, item, source, actions, people, cardFocus }: ActionItemRowProps) {
    const titleId = useId();
    const mark = useMarkActionItem(boardId);
    const edit = useEditActionItem(boardId);
    const removal = useDeleteActionItem(boardId);
    const [dialog, setDialog] = useState<Dialog>(null);

    const due = formatDueDate(item.due_on);
    const toggled = nextStatus(item.status);
    const items: MenuItem[] = [];

    if (actions.edit) {
        items.push({
            label: "Edit action item",
            onSelect: () => {
                edit.reset();
                setDialog("edit");
            },
        });
    }

    if (actions.mark && item.status !== "dropped") {
        items.push({ label: "Drop", onSelect: () => mark.mutate({ itemId: item.id, status: "dropped" }) });
    }

    if (canRemoveActionItem(actions, item)) {
        items.push({
            label: "Delete action item",
            tone: "danger",
            onSelect: () => {
                removal.reset();
                setDialog("delete");
            },
        });
    }

    function handleSave(fields: ActionItemSubmission) {
        edit.mutate({ itemId: item.id, ...fields }, { onSuccess: () => setDialog(null) });
    }

    function handleDelete() {
        removal.mutate({ itemId: item.id }, { onSuccess: () => setDialog(null) });
    }

    const deleteBody = (
        <div className={styles.dialog}>
            <p>The action item is deleted for everyone, here and on the Action Items page. This cannot be undone.</p>
            {removal.error !== null && <ErrorBanner message={removal.error.message} takeFocus />}
        </div>
    );

    return (
        <li className={styles.item} aria-labelledby={titleId}>
            <div className={styles.itemMain}>
                <span className={item.status === "open" ? styles.itemTitle : `${styles.itemTitle} ${styles.closed}`}>
                    <MentionText id={titleId} text={item.title} mentions={item.mentions} />
                </span>
                <span className={styles.itemMeta}>
                    {item.assignee === null ? "No one assigned" : <bdi>{item.assignee.display_name}</bdi>}
                    {due !== "" && ` · Due ${due}`}
                </span>
                {source !== null && (
                    <Button
                        type="button"
                        size="small"
                        variant="ghost"
                        className={styles.source}
                        onClick={() => cardFocus.reveal(revealedCardId(source))}
                    >
                        <span className={styles.sourceText} dir="auto">
                            From: {source.body}
                        </span>
                    </Button>
                )}
            </div>
            <div className={styles.itemSide}>
                <StatusTag tone={STATUS_TONES[item.status]}>{ACTION_ITEM_STATUS_LABELS[item.status]}</StatusTag>
                {actions.mark && (
                    <Button
                        type="button"
                        size="small"
                        disabled={mark.isPending}
                        onClick={() => mark.mutate({ itemId: item.id, status: toggled })}
                    >
                        {toggled === "done" ? "Mark done" : "Reopen"}
                    </Button>
                )}
                {items.length > 0 && <Menu label="Action item options" items={items} iconOnly size="small" />}
            </div>
            <Modal isOpen={dialog === "edit"} onClose={() => setDialog(null)} title="Edit action item">
                <ActionItemForm
                    label="Edit action item"
                    initial={{
                        title: item.title,
                        assignee: item.assignee,
                        dueOn: item.due_on,
                        mentions: item.mentions,
                    }}
                    people={actions.name ? people : null}
                    submitLabel="Save action item"
                    pending={edit.isPending}
                    error={edit.error}
                    onSubmit={handleSave}
                    onCancel={() => setDialog(null)}
                />
            </Modal>
            <ConfirmDialog
                open={dialog === "delete"}
                title="Delete this action item?"
                body={deleteBody}
                confirmLabel="Delete action item"
                destructive
                busy={removal.isPending}
                onConfirm={handleDelete}
                onCancel={() => setDialog(null)}
            />
            {mark.error !== null && (
                <Toast variant="error" onDismiss={mark.reset}>
                    {mark.error.message}
                </Toast>
            )}
        </li>
    );
}
