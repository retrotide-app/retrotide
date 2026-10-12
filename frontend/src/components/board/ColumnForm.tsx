import { type SubmitEvent, useState } from "react";
import {
    BOARD_COLOURS,
    COLOUR_LABELS,
    COLUMN_DESCRIPTION_MAX_LENGTH,
    COLUMN_TITLE_MAX_LENGTH,
} from "../../domain/board/fields";
import {
    COLOUR_REFUSALS,
    COLUMN_DESCRIPTION_REFUSALS,
    COLUMN_TITLE_REFUSALS,
    fieldRefusal,
} from "../../domain/refusals";
import type { Colour } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { FormField } from "../FormField/FormField";
import { TextField } from "../FormField/TextField";
import { Select } from "../Select/Select";
import styles from "./boardForm.module.css";

export interface ColumnFields {
    title: string;
    description: string;
    colour: Colour;
}

interface ColumnFormProps {
    label: string;
    initial: ColumnFields;
    submitLabel: string;
    pending: boolean;
    error: Error | null;
    onSubmit: (fields: ColumnFields) => void;
    onCancel: () => void;
}

const TITLE_MISSING = "Enter a column title.";

export function ColumnForm({ label, initial, submitLabel, pending, error, onSubmit, onCancel }: ColumnFormProps) {
    const [title, setTitle] = useState(initial.title);
    const [description, setDescription] = useState(initial.description);
    const [colour, setColour] = useState<Colour>(initial.colour);
    const [problem, setProblem] = useState<string | null>(null);

    const titleError = problem ?? fieldRefusal(error, COLUMN_TITLE_REFUSALS);
    const descriptionError = fieldRefusal(error, COLUMN_DESCRIPTION_REFUSALS);
    const colourError = fieldRefusal(error, COLOUR_REFUSALS);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (title.trim() === "") {
            setProblem(TITLE_MISSING);
            return;
        }

        setProblem(null);
        onSubmit({ title, description, colour });
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} aria-label={label} noValidate>
            {problem !== null && <ErrorBanner message={problem} takeFocus />}
            {problem === null && error !== null && <ErrorBanner message={error.message} takeFocus />}
            <TextField
                label="Column title"
                hint={`Up to ${COLUMN_TITLE_MAX_LENGTH} characters.`}
                error={titleError}
                value={title}
                onChange={e => setTitle(e.target.value)}
                maxLength={COLUMN_TITLE_MAX_LENGTH}
                autoComplete="off"
                required
            />
            <TextField
                label="Description"
                hint={`Optional. Up to ${COLUMN_DESCRIPTION_MAX_LENGTH} characters.`}
                error={descriptionError}
                value={description}
                onChange={e => setDescription(e.target.value)}
                maxLength={COLUMN_DESCRIPTION_MAX_LENGTH}
                autoComplete="off"
            />
            <FormField label="Column colour" error={colourError}>
                {control => (
                    <Select
                        {...control}
                        size="small"
                        value={colour}
                        onChange={e => setColour(BOARD_COLOURS.find(option => option === e.target.value) ?? colour)}
                    >
                        {BOARD_COLOURS.map(option => (
                            <option key={option} value={option}>
                                {COLOUR_LABELS[option]}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
            <div className={styles.actions}>
                <Button type="submit" variant="primary" size="small" disabled={pending}>
                    {submitLabel}
                </Button>
                <Button type="button" variant="ghost" size="small" onClick={onCancel} disabled={pending}>
                    Cancel
                </Button>
            </div>
        </form>
    );
}
