import { type FormEvent, useId, useRef, useState } from "react";
import {
    draftRequest,
    emptySurveyDraft,
    newQuestionDraft,
    QUESTION_KINDS,
    type QuestionDraft,
    SURVEY_MAX_OPTIONS,
    SURVEY_MAX_QUESTIONS,
    SURVEY_MIN_OPTIONS,
    SURVEY_SCALE_CEILING,
    SURVEY_SCALE_FLOOR,
    surveyRefusal,
    templatesByCategory,
    validateDraft,
    withKind,
} from "../../../domain/board/surveys";
import { useCreateSurvey } from "../../../hooks/mutations/surveys";
import { useSurveyTemplates } from "../../../hooks/queries/surveys";
import type { SurveyQuestionKind } from "../../../types/api";
import { Button } from "../../Button/Button";
import { Input } from "../../Input/Input";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { Modal } from "../../Modal/Modal";
import { Select } from "../../Select/Select";
import styles from "./Surveys.module.css";

interface NewSurveyDialogProps {
    boardId: string;
    onClose: () => void;
}

interface DraftRow {
    key: number;
    question: QuestionDraft;
}

interface QuestionEditorProps {
    position: number;
    question: QuestionDraft;
    removable: boolean;
    onChange: (question: QuestionDraft) => void;
    onRemove: () => void;
}

function shownScale(value: number): number | "" {
    return Number.isNaN(value) ? "" : value;
}

function QuestionEditor({ position, question, removable, onChange, onRemove }: QuestionEditorProps) {
    const kindId = useId();

    return (
        <fieldset className={styles.questionRow}>
            <legend className={styles.questionLegend}>{`Question ${position}`}</legend>
            <div className={styles.field}>
                <label htmlFor={kindId} className={styles.fieldLabel}>
                    Kind
                </label>
                <Select
                    id={kindId}
                    value={question.kind}
                    onChange={event => onChange(withKind(question, event.target.value as SurveyQuestionKind))}
                >
                    {QUESTION_KINDS.map(kind => (
                        <option key={kind.value} value={kind.value}>
                            {kind.label}
                        </option>
                    ))}
                </Select>
            </div>
            <label className={styles.field}>
                <span className={styles.fieldLabel}>Prompt</span>
                <Input
                    fullWidth
                    value={question.prompt}
                    onChange={event => onChange({ ...question, prompt: event.target.value })}
                />
            </label>
            <div className={styles.questionDetail}>
                {question.kind === "scale" ? (
                    <div className={styles.scaleRow}>
                        <label className={styles.field}>
                            <span className={styles.fieldLabel}>Lowest score</span>
                            <Input
                                type="number"
                                min={SURVEY_SCALE_FLOOR}
                                max={SURVEY_SCALE_CEILING}
                                className={styles.scaleInput}
                                value={shownScale(question.scale_min)}
                                onChange={event =>
                                    onChange({ ...question, scale_min: Number.parseInt(event.target.value, 10) })
                                }
                            />
                        </label>
                        <label className={styles.field}>
                            <span className={styles.fieldLabel}>Highest score</span>
                            <Input
                                type="number"
                                min={SURVEY_SCALE_FLOOR}
                                max={SURVEY_SCALE_CEILING}
                                className={styles.scaleInput}
                                value={shownScale(question.scale_max)}
                                onChange={event =>
                                    onChange({ ...question, scale_max: Number.parseInt(event.target.value, 10) })
                                }
                            />
                        </label>
                    </div>
                ) : (
                    <>
                        {question.options.map((option, index) => (
                            <div key={index} className={styles.optionRow}>
                                <Input
                                    aria-label={`Option ${index + 1}`}
                                    className={styles.optionInput}
                                    value={option}
                                    onChange={event =>
                                        onChange({
                                            ...question,
                                            options: question.options.map((each, at) =>
                                                at === index ? event.target.value : each,
                                            ),
                                        })
                                    }
                                />
                                <Button
                                    type="button"
                                    size="small"
                                    variant="ghost"
                                    aria-label={`Remove option ${index + 1}`}
                                    disabled={question.options.length <= SURVEY_MIN_OPTIONS}
                                    onClick={() =>
                                        onChange({
                                            ...question,
                                            options: question.options.filter((_, at) => at !== index),
                                        })
                                    }
                                >
                                    Remove
                                </Button>
                            </div>
                        ))}
                        <div className={styles.builderButtons}>
                            <Button
                                type="button"
                                size="small"
                                disabled={question.options.length >= SURVEY_MAX_OPTIONS}
                                onClick={() => onChange({ ...question, options: [...question.options, ""] })}
                            >
                                Add option
                            </Button>
                        </div>
                    </>
                )}
                {removable && (
                    <div className={styles.builderButtons}>
                        <Button type="button" size="small" variant="ghost" onClick={onRemove}>
                            Remove question
                        </Button>
                    </div>
                )}
            </div>
        </fieldset>
    );
}

export function NewSurveyDialog({ boardId, onClose }: NewSurveyDialogProps) {
    const create = useCreateSurvey(boardId);
    const templates = useSurveyTemplates();
    const [custom, setCustom] = useState(false);
    const [title, setTitle] = useState("");
    const [rows, setRows] = useState<DraftRow[]>(() =>
        emptySurveyDraft().questions.map((question, key) => ({ key, question })),
    );
    const [attempted, setAttempted] = useState(false);
    const nextKey = useRef(rows.length);

    const draft = { title, questions: rows.map(row => row.question) };
    const refusal = attempted ? validateDraft(draft) : null;
    const failure = create.error === null ? null : surveyRefusal(create.error);

    function addQuestion() {
        const key = nextKey.current;
        nextKey.current += 1;

        setRows(current => [...current, { key, question: newQuestionDraft("single_choice") }]);
    }

    function submit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        setAttempted(true);

        if (validateDraft(draft) !== null) {
            return;
        }

        create.mutate(draftRequest(draft), { onSuccess: onClose });
    }

    return (
        <Modal isOpen onClose={onClose} title="New survey">
            {custom ? (
                <form className={styles.builder} onSubmit={submit} aria-label="Custom survey" noValidate>
                    <label className={styles.field}>
                        <span className={styles.fieldLabel}>Title</span>
                        <Input fullWidth value={title} onChange={event => setTitle(event.target.value)} />
                    </label>
                    {rows.map((row, index) => (
                        <QuestionEditor
                            key={row.key}
                            position={index + 1}
                            question={row.question}
                            removable={rows.length > 1}
                            onChange={question =>
                                setRows(current =>
                                    current.map(each => (each.key === row.key ? { ...each, question } : each)),
                                )
                            }
                            onRemove={() => setRows(current => current.filter(each => each.key !== row.key))}
                        />
                    ))}
                    {(refusal ?? failure) !== null && (
                        <p role="alert" className={styles.error}>
                            {refusal ?? failure}
                        </p>
                    )}
                    <div className={styles.builderActions}>
                        <Button
                            type="button"
                            size="small"
                            disabled={rows.length >= SURVEY_MAX_QUESTIONS}
                            onClick={addQuestion}
                        >
                            Add question
                        </Button>
                        <div className={styles.builderButtons}>
                            <Button type="button" onClick={() => setCustom(false)}>
                                Back
                            </Button>
                            <Button type="submit" variant="primary" disabled={create.isPending}>
                                Create survey
                            </Button>
                        </div>
                    </div>
                </form>
            ) : (
                <div className={styles.dialog}>
                    {templates.error !== null && (
                        <LoadFailure message={templates.error.message} onRetry={templates.refetch} />
                    )}
                    {templates.loading && <p className={styles.note}>Loading the templates…</p>}
                    {templatesByCategory(templates.data?.templates ?? []).map(group => (
                        <section key={group.category} className={styles.group} aria-label={group.label}>
                            <p className={styles.groupLabel}>{group.label}</p>
                            <div className={styles.tiles}>
                                {group.templates.map(template => (
                                    <Button
                                        key={template.key}
                                        type="button"
                                        className={styles.tile}
                                        disabled={create.isPending}
                                        onClick={() =>
                                            create.mutate(
                                                { title: template.title, template_key: template.key },
                                                { onSuccess: onClose },
                                            )
                                        }
                                    >
                                        <span className={styles.tileTitle}>{template.title}</span>
                                        <span className={styles.tileText}>{template.description}</span>
                                    </Button>
                                ))}
                            </div>
                        </section>
                    ))}
                    <section className={styles.group} aria-label="Your own">
                        <p className={styles.groupLabel}>Your own</p>
                        <div className={styles.tiles}>
                            <Button type="button" className={styles.tile} onClick={() => setCustom(true)}>
                                <span className={styles.tileTitle}>Custom survey</span>
                                <span className={styles.tileText}>
                                    Write your own questions: single choice, multiple choice or a scale.
                                </span>
                            </Button>
                        </div>
                    </section>
                    {failure !== null && (
                        <p role="alert" className={styles.error}>
                            {failure}
                        </p>
                    )}
                </div>
            )}
        </Modal>
    );
}
