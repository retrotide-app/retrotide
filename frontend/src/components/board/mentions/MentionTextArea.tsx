import type { ComponentProps, KeyboardEvent } from "react";
import { mentionPlaceholder } from "../../../domain/board/mentions";
import { useBoardMembership } from "../../../hooks/board/useBoardMembership";
import { useMentionPicker } from "../../../hooks/board/useMentionPicker";
import type { PersonView } from "../../../types/api";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { TextArea } from "../../TextArea/TextArea";
import { MentionSuggestions } from "./MentionSuggestions";
import styles from "./Mentions.module.css";

type TextAreaPassthrough = Omit<
    ComponentProps<typeof TextArea>,
    "ref" | "role" | "value" | "onChange" | "onSelect" | "onKeyDown" | "onBlur" | "onFocus" | "autoComplete"
>;

interface MentionTextAreaProps extends TextAreaPassthrough {
    boardId: string;
    text: string;
    mentions: readonly PersonView[];
    onTextChange: (text: string, mentions: PersonView[]) => void;
    onKeyDown?: (e: KeyboardEvent<HTMLTextAreaElement>) => void;
}

export function MentionTextArea({
    boardId,
    text,
    mentions,
    onTextChange,
    onKeyDown,
    placeholder,
    ...rest
}: MentionTextAreaProps) {
    const membership = useBoardMembership(boardId, true);
    const picker = useMentionPicker<HTMLTextAreaElement>({
        text,
        mentions,
        people: membership.people,
        onChange: onTextChange,
        onKeyDown,
    });

    const picking = membership.people !== null;
    const {
        ref,
        value,
        onChange,
        onSelect,
        onKeyDown: handleKeyDown,
        onBlur,
        onFocus,
        ...comboboxRole
    } = picker.combobox;

    return (
        <div className={styles.field}>
            <TextArea
                {...rest}
                {...(picking ? comboboxRole : {})}
                ref={ref}
                value={value}
                onChange={onChange}
                onSelect={onSelect}
                onKeyDown={handleKeyDown}
                onBlur={onBlur}
                onFocus={onFocus}
                placeholder={mentionPlaceholder(placeholder, picking)}
            />
            <MentionSuggestions picker={picker} />
            {membership.failure !== null && (
                <LoadFailure message={membership.failure.message} onRetry={membership.retry} />
            )}
        </div>
    );
}
