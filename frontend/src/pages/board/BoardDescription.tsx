import { type SubmitEvent, useState } from "react";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormField } from "../../components/FormField/FormField";
import { TextArea } from "../../components/TextArea/TextArea";
import { BOARD_CONTEXT_MAX_LENGTH } from "../../domain/board/fields";
import { BOARD_CONTEXT_REFUSALS, fieldRefusal } from "../../domain/refusals";
import { useDescribeBoard } from "../../hooks/mutations/boards";
import type { BoardView } from "../../types/api";
import styles from "../org/shared/OrgSection.module.css";
import boardStyles from "./BoardPage.module.css";

interface BoardDescriptionProps {
    board: BoardView;
    editable: boolean;
}

export function BoardDescription({ board, editable }: BoardDescriptionProps) {
    const describe = useDescribeBoard(board.id);
    const [editing, setEditing] = useState(false);
    const [context, setContext] = useState(board.context);

    function startEditing() {
        describe.reset();
        setContext(board.context);
        setEditing(true);
    }

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        describe.mutate(
            { context },
            {
                onSuccess: () => {
                    setEditing(false);
                },
            },
        );
    }

    if (editing) {
        return (
            <form className={styles.form} onSubmit={handleSubmit} aria-label="Board description" noValidate>
                {describe.error !== null && <ErrorBanner message={describe.error.message} takeFocus />}
                <FormField
                    label="Description"
                    hint={`Up to ${BOARD_CONTEXT_MAX_LENGTH} characters. Leave it empty to remove the description.`}
                    error={fieldRefusal(describe.error, BOARD_CONTEXT_REFUSALS)}
                >
                    {control => (
                        <TextArea
                            {...control}
                            value={context}
                            onChange={e => setContext(e.target.value)}
                            maxLength={BOARD_CONTEXT_MAX_LENGTH}
                            rows={3}
                        />
                    )}
                </FormField>
                <div className={styles.actions}>
                    <Button type="submit" variant="primary" size="small" disabled={describe.isPending}>
                        Save description
                    </Button>
                    <Button
                        type="button"
                        variant="ghost"
                        size="small"
                        onClick={() => setEditing(false)}
                        disabled={describe.isPending}
                    >
                        Cancel
                    </Button>
                </div>
            </form>
        );
    }

    if (board.context === "" && !editable) {
        return null;
    }

    return (
        <div className={styles.inline}>
            {board.context !== "" && (
                <p className={boardStyles.description} dir="auto">
                    {board.context}
                </p>
            )}
            {board.context === "" && <p className={boardStyles.muted}>No description yet.</p>}
            {editable && (
                <Button type="button" size="small" variant="ghost" onClick={startEditing}>
                    Edit description
                </Button>
            )}
        </div>
    );
}
