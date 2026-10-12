import type { MentionElement, MentionPicker } from "../../../hooks/board/useMentionPicker";
import styles from "./Mentions.module.css";

interface MentionSuggestionsProps<T extends MentionElement> {
    picker: MentionPicker<T>;
}

export function MentionSuggestions<T extends MentionElement>({ picker }: MentionSuggestionsProps<T>) {
    const { open, listId, suggestions, active, optionIds, pick } = picker;

    if (!open) {
        return null;
    }

    return (
        <ul id={listId} role="listbox" aria-label="People to mention" className={styles.suggestions}>
            {suggestions.map((person, index) => {
                const current = index === active;

                return (
                    <li
                        key={person.user_id}
                        id={optionIds[index]}
                        role="option"
                        aria-selected={current}
                        className={current ? `${styles.option} ${styles.active}` : styles.option}
                        onMouseDown={e => e.preventDefault()}
                        onClick={() => pick(person)}
                    >
                        <bdi>{person.display_name}</bdi>
                    </li>
                );
            })}
        </ul>
    );
}
