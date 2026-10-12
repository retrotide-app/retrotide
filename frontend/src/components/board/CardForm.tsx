import { type SubmitEvent, useState } from "react";
import { CARD_BODY_MAX_LENGTH, BOARD_COLOURS, COLOUR_LABELS } from "../../domain/board/fields";
import { keptMentions } from "../../domain/board/mentions";
import { CARD_BODY_REFUSALS, COLOUR_REFUSALS, fieldRefusal, MENTION_REFUSALS } from "../../domain/refusals";
import type { Colour, PersonView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { FormField } from "../FormField/FormField";
import { Select } from "../Select/Select";
import styles from "./boardForm.module.css";
import { MentionTextArea } from "./mentions/MentionTextArea";

export interface CardInitial {
    body: string;
    colour: Colour | null;
    mentions: readonly PersonView[];
}

export interface CardFields {
    body: string;
    colour: Colour | null;
    mentions: string[];
}

interface CardFormProps {
    boardId: string;
    label: string;
    initial: CardInitial;
    submitLabel: string;
    pending: boolean;
    error: Error | null;
    onSubmit: (fields: CardFields) => void;
    onCancel: () => void;
}

const BODY_MISSING = "Write something on the card first.";

const NO_COLOUR = "";

function colourFrom(value: string): Colour | null {
    return BOARD_COLOURS.find(colour => colour === value) ?? null;
}

export function CardForm({ boardId, label, initial, submitLabel, pending, error, onSubmit, onCancel }: CardFormProps) {
    const [body, setBody] = useState(initial.body);
    const [mentions, setMentions] = useState<readonly PersonView[]>(initial.mentions);
    const [colour, setColour] = useState<Colour | null>(initial.colour);
    const [problem, setProblem] = useState<string | null>(null);

    const bodyError = problem ?? fieldRefusal(error, CARD_BODY_REFUSALS) ?? fieldRefusal(error, MENTION_REFUSALS);
    const colourError = fieldRefusal(error, COLOUR_REFUSALS);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (body.trim() === "") {
            setProblem(BODY_MISSING);
            return;
        }

        setProblem(null);
        onSubmit({ body, colour, mentions: keptMentions(body, mentions).map(person => person.user_id) });
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} aria-label={label} noValidate>
            {problem !== null && <ErrorBanner message={problem} takeFocus />}
            {problem === null && error !== null && <ErrorBanner message={error.message} takeFocus />}
            <FormField label="Card text" hint={`Up to ${CARD_BODY_MAX_LENGTH} characters.`} error={bodyError}>
                {control => (
                    <MentionTextArea
                        {...control}
                        boardId={boardId}
                        text={body}
                        mentions={mentions}
                        onTextChange={(nextBody, nextMentions) => {
                            setBody(nextBody);
                            setMentions(nextMentions);
                        }}
                        maxLength={CARD_BODY_MAX_LENGTH}
                        rows={3}
                        required
                    />
                )}
            </FormField>
            <FormField label="Card colour" error={colourError}>
                {control => (
                    <Select
                        {...control}
                        size="small"
                        value={colour ?? NO_COLOUR}
                        onChange={e => setColour(colourFrom(e.target.value))}
                    >
                        <option value={NO_COLOUR}>No colour</option>
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
