import { ACTION_ITEM_TITLE_MAX_LENGTH } from "../../../domain/board/actionItems";
import { mentionPlaceholder } from "../../../domain/board/mentions";
import { useMentionPicker } from "../../../hooks/board/useMentionPicker";
import type { PersonView } from "../../../types/api";
import { FormField } from "../../FormField/FormField";
import { Input } from "../../Input/Input";
import { MentionSuggestions } from "../mentions/MentionSuggestions";
import mentionStyles from "../mentions/Mentions.module.css";

interface MentionTitleFieldProps {
    label: string;
    title: string;
    mentions: readonly PersonView[];
    people: readonly PersonView[] | null;
    error?: string | null;
    hint?: string;
    autoFocus?: boolean;
    onChange: (title: string, mentions: PersonView[]) => void;
}

const PLACEHOLDER = "What needs doing?";

export function MentionTitleField({
    label,
    title,
    mentions,
    people,
    error,
    hint,
    autoFocus,
    onChange,
}: MentionTitleFieldProps) {
    const picker = useMentionPicker<HTMLInputElement>({ text: title, mentions, people, onChange });

    return (
        <FormField label={label} hint={hint} error={error}>
            {control => (
                <div className={mentionStyles.field}>
                    <Input
                        {...control}
                        {...picker.combobox}
                        fullWidth
                        autoFocus={autoFocus}
                        maxLength={ACTION_ITEM_TITLE_MAX_LENGTH}
                        placeholder={mentionPlaceholder(PLACEHOLDER, people !== null)}
                    />
                    <MentionSuggestions picker={picker} />
                </div>
            )}
        </FormField>
    );
}
