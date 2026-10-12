import { type SubmitEvent, useState } from "react";
import { cardMoveChoices } from "../../domain/board/drop";
import type { CardFocus } from "../../hooks/board/useCardFocus";
import { useMoveCard } from "../../hooks/mutations/boards";
import type { BoardSnapshot, CardView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { FormField } from "../FormField/FormField";
import { Select } from "../Select/Select";
import styles from "./boardForm.module.css";

interface CardMoveFormProps {
    snapshot: BoardSnapshot;
    card: CardView;
    cardFocus: CardFocus;
}

export function CardMoveForm({ snapshot, card, cardFocus }: CardMoveFormProps) {
    const move = useMoveCard(snapshot.board_id);
    const choices = cardMoveChoices(snapshot, card.id);
    const [chosenKey, setChosenKey] = useState("");

    const chosen = choices.find(choice => choice.key === chosenKey) ?? choices[0];

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (chosen === undefined) {
            return;
        }

        if (chosen.drop.column_id !== card.column_id) {
            cardFocus.request(card.id);
        }

        move.mutate(
            { cardId: card.id, ...chosen.drop },
            {
                onError: () => {
                    cardFocus.request(null);
                },
            },
        );
    }

    if (chosen === undefined) {
        return null;
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} aria-label="Move card" noValidate>
            {move.error !== null && <ErrorBanner message={move.error.message} takeFocus />}
            <FormField label="Move to">
                {control => (
                    <Select {...control} size="small" value={chosen.key} onChange={e => setChosenKey(e.target.value)}>
                        {choices.map(choice => (
                            <option key={choice.key} value={choice.key}>
                                {choice.label}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
            <div className={styles.actions}>
                <Button type="submit" size="small" disabled={move.isPending}>
                    Move card
                </Button>
            </div>
        </form>
    );
}
