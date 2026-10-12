import { type KeyboardEvent, type SubmitEvent, useState } from "react";
import { CARD_BODY_MAX_LENGTH } from "../../domain/board/fields";
import { keptMentions } from "../../domain/board/mentions";
import { useCreateCard } from "../../hooks/mutations/boards";
import type { ColumnView, PersonView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./BoardColumn.module.css";
import { MentionTextArea } from "./mentions/MentionTextArea";

interface AddCardFormProps {
    boardId: string;
    column: ColumnView;
}

const BODY_MISSING = "Write something on the card first.";

export function AddCardForm({ boardId, column }: AddCardFormProps) {
    const create = useCreateCard(boardId);
    const [open, setOpen] = useState(false);
    const [body, setBody] = useState("");
    const [mentions, setMentions] = useState<readonly PersonView[]>([]);
    const [problem, setProblem] = useState<string | null>(null);

    function clear() {
        setBody("");
        setMentions([]);
    }

    function openForm() {
        create.reset();
        setProblem(null);
        setOpen(true);
    }

    function close() {
        clear();
        setProblem(null);
        setOpen(false);
    }

    function submit() {
        if (body.trim() === "") {
            setProblem(BODY_MISSING);
            return;
        }

        setProblem(null);
        create.mutate(
            {
                column_id: column.id,
                body,
                colour: null,
                mentions: keptMentions(body, mentions).map(person => person.user_id),
            },
            { onSuccess: clear },
        );
    }

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        submit();
    }

    function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
        if (e.key === "Escape") {
            close();
            return;
        }

        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            submit();
        }
    }

    if (!open) {
        return (
            <Button type="button" variant="ghost" className={styles.add} onClick={openForm} data-new-card="">
                <span aria-hidden="true">+</span> Add a card
            </Button>
        );
    }

    const message = problem ?? create.error?.message ?? null;

    return (
        <form
            className={styles.addForm}
            onSubmit={handleSubmit}
            aria-label={`Add a card to ${column.title}`}
            noValidate
            data-new-card=""
        >
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <MentionTextArea
                boardId={boardId}
                aria-label="Card text"
                placeholder="Write a card, then press Enter"
                text={body}
                mentions={mentions}
                onTextChange={(nextBody, nextMentions) => {
                    setBody(nextBody);
                    setMentions(nextMentions);
                }}
                onKeyDown={handleKeyDown}
                maxLength={CARD_BODY_MAX_LENGTH}
                rows={2}
                autoFocus
            />
            <div className={styles.addActions}>
                <Button type="submit" size="small" variant="primary" disabled={create.isPending}>
                    Add card
                </Button>
                <Button type="button" size="small" variant="ghost" onClick={close}>
                    Cancel
                </Button>
            </div>
        </form>
    );
}
