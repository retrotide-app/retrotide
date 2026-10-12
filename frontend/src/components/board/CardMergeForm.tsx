import { type SubmitEvent, useState } from "react";
import { mergeChoices } from "../../domain/board/drop";
import { useMergeCard } from "../../hooks/mutations/boards";
import type { BoardSnapshot, CardView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { FormField } from "../FormField/FormField";
import { Select } from "../Select/Select";
import styles from "./boardForm.module.css";

interface CardMergeFormProps {
    snapshot: BoardSnapshot;
    card: CardView;
    onMerged: () => void;
}

export function CardMergeForm({ snapshot, card, onMerged }: CardMergeFormProps) {
    const merge = useMergeCard(snapshot.board_id);
    const choices = mergeChoices(snapshot, card.id);
    const [chosenId, setChosenId] = useState("");

    const chosen = choices.find(choice => choice.cardId === chosenId) ?? choices[0];

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (chosen === undefined) {
            return;
        }

        merge.mutate({ cardId: card.id, targetCardId: chosen.cardId }, { onSuccess: onMerged });
    }

    if (chosen === undefined) {
        return <p>There is no other card to merge this one into.</p>;
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} aria-label="Merge card" noValidate>
            {merge.error !== null && <ErrorBanner message={merge.error.message} takeFocus />}
            <FormField
                label="Merge into"
                hint="This card joins the one you pick, with its votes and comments, and can be taken out again."
            >
                {control => (
                    <Select {...control} size="small" value={chosen.cardId} onChange={e => setChosenId(e.target.value)}>
                        {choices.map(choice => (
                            <option key={choice.cardId} value={choice.cardId}>
                                {choice.label}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
            <div className={styles.actions}>
                <Button type="submit" size="small" variant="primary" disabled={merge.isPending}>
                    Merge cards
                </Button>
            </div>
        </form>
    );
}
