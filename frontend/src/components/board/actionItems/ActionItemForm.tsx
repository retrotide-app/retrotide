import { type SubmitEvent, useState } from "react";
import { dueOnFrom } from "../../../domain/board/actionItems";
import { keptMentions } from "../../../domain/board/mentions";
import {
    ACTION_ITEM_TITLE_REFUSALS,
    DUE_DATE_REFUSALS,
    fieldRefusal,
    NAMED_PERSON_REFUSALS,
} from "../../../domain/refusals";
import type { ActionItemFieldsRequest, PersonView } from "../../../types/api";
import { Button } from "../../Button/Button";
import { ErrorBanner } from "../../ErrorBanner/ErrorBanner";
import { FormField } from "../../FormField/FormField";
import { Input } from "../../Input/Input";
import { Select } from "../../Select/Select";
import styles from "./ActionItems.module.css";
import { MentionTitleField } from "./MentionTitleField";

export type ActionItemSubmission = Omit<ActionItemFieldsRequest, "op_id">;

export interface ActionItemInitial {
    title: string;
    assignee: PersonView | null;
    dueOn: string | null;
    mentions: readonly PersonView[];
}

interface ActionItemFormProps {
    label: string;
    initial: ActionItemInitial;
    people: readonly PersonView[] | null;
    submitLabel: string;
    pending: boolean;
    error: Error | null;
    autoFocus?: boolean;
    onSubmit: (fields: ActionItemSubmission) => void;
    onCancel?: () => void;
}

const NOBODY = "";

const TITLE_MISSING = "Write what needs doing first.";

function assigneeOptions(people: readonly PersonView[], held: PersonView | null): readonly PersonView[] {
    if (held === null || people.some(person => person.user_id === held.user_id)) {
        return people;
    }

    return [...people, held];
}

export function ActionItemForm({
    label,
    initial,
    people,
    submitLabel,
    pending,
    error,
    autoFocus,
    onSubmit,
    onCancel,
}: ActionItemFormProps) {
    const [title, setTitle] = useState(initial.title);
    const [mentions, setMentions] = useState<readonly PersonView[]>(initial.mentions);
    const [assigneeId, setAssigneeId] = useState(initial.assignee?.user_id ?? NOBODY);
    const [dueOn, setDueOn] = useState(initial.dueOn ?? "");
    const [problem, setProblem] = useState<string | null>(null);

    const titleError = problem ?? fieldRefusal(error, ACTION_ITEM_TITLE_REFUSALS);
    const personError = fieldRefusal(error, NAMED_PERSON_REFUSALS);
    const dueError = fieldRefusal(error, DUE_DATE_REFUSALS);
    const fieldError = titleError ?? personError ?? dueError;
    const formError = problem === null && fieldError === null && error !== null ? error.message : null;

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (title.trim() === "") {
            setProblem(TITLE_MISSING);
            return;
        }

        setProblem(null);
        onSubmit({
            title,
            assignee_user_id: assigneeId === NOBODY ? null : assigneeId,
            due_on: dueOnFrom(dueOn),
            mentions: keptMentions(title, mentions).map(person => person.user_id),
        });
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} aria-label={label} noValidate>
            {formError !== null && <ErrorBanner message={formError} takeFocus />}
            <div className={styles.fields}>
                <div className={styles.titleField}>
                    <MentionTitleField
                        label="Action item"
                        title={title}
                        mentions={mentions}
                        people={people}
                        error={titleError}
                        autoFocus={autoFocus}
                        onChange={(nextTitle, nextMentions) => {
                            setTitle(nextTitle);
                            setMentions(nextMentions);
                        }}
                    />
                </div>
                <div className={styles.assigneeField}>
                    {people === null ? (
                        <FormField label="Assignee">
                            {control => (
                                <Input
                                    {...control}
                                    fullWidth
                                    readOnly
                                    value={initial.assignee?.display_name ?? "No one"}
                                />
                            )}
                        </FormField>
                    ) : (
                        <FormField label="Assignee" error={personError}>
                            {control => (
                                <Select {...control} value={assigneeId} onChange={e => setAssigneeId(e.target.value)}>
                                    <option value={NOBODY}>No one</option>
                                    {assigneeOptions(people, initial.assignee).map(person => (
                                        <option key={person.user_id} value={person.user_id}>
                                            {person.display_name}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </FormField>
                    )}
                </div>
                <div className={styles.dueField}>
                    <FormField label="Due date" error={dueError}>
                        {control => (
                            <Input
                                {...control}
                                fullWidth
                                type="date"
                                value={dueOn}
                                onChange={e => setDueOn(e.target.value)}
                            />
                        )}
                    </FormField>
                </div>
                <div className={styles.formActions}>
                    <Button type="submit" variant="primary" size="small" disabled={pending}>
                        {submitLabel}
                    </Button>
                    {onCancel !== undefined && (
                        <Button type="button" variant="ghost" size="small" onClick={onCancel} disabled={pending}>
                            Cancel
                        </Button>
                    )}
                </div>
            </div>
        </form>
    );
}
