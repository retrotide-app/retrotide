import { useState } from "react";
import { DEFAULT_COLUMN_COLOUR } from "../../domain/board/fields";
import { useCreateColumn } from "../../hooks/mutations/boards";
import { Button } from "../Button/Button";
import { ColumnForm, type ColumnFields } from "./ColumnForm";

interface AddColumnFormProps {
    boardId: string;
}

const EMPTY_COLUMN: ColumnFields = { title: "", description: "", colour: DEFAULT_COLUMN_COLOUR };

export function AddColumnForm({ boardId }: AddColumnFormProps) {
    const create = useCreateColumn(boardId);
    const [open, setOpen] = useState(false);

    function openForm() {
        create.reset();
        setOpen(true);
    }

    function handleSubmit(fields: ColumnFields) {
        create.mutate(fields, {
            onSuccess: () => {
                setOpen(false);
            },
        });
    }

    if (!open) {
        return (
            <div>
                <Button type="button" onClick={openForm}>
                    Add a column
                </Button>
            </div>
        );
    }

    return (
        <ColumnForm
            label="Add a column"
            initial={EMPTY_COLUMN}
            submitLabel="Add column"
            pending={create.isPending}
            error={create.error}
            onSubmit={handleSubmit}
            onCancel={() => setOpen(false)}
        />
    );
}
