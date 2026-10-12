import { type FormEvent, useId, useState } from "react";
import {
    answersComplete,
    pick,
    respondentsLabel,
    scaleValues,
    surveyAnswers,
    surveyRefusal,
} from "../../../domain/board/surveys";
import { useCloseSurvey, useDeleteSurvey, useRespondToSurvey } from "../../../hooks/mutations/surveys";
import type { SurveyQuestionView, SurveyView } from "../../../types/api";
import { Button } from "../../Button/Button";
import { ConfirmDialog } from "../../ConfirmDialog/ConfirmDialog";
import { Input } from "../../Input/Input";
import { StatusTag } from "../../StatusTag/StatusTag";
import { SurveyResults } from "./SurveyResults";
import styles from "./Surveys.module.css";

interface SurveyCardProps {
    boardId: string;
    survey: SurveyView;
    canRespond: boolean;
    canManage: boolean;
}

interface SurveyQuestionProps {
    question: SurveyQuestionView;
    picked: readonly number[];
    onPick: (value: number) => void;
}

function SurveyQuestion({ question, picked, onPick }: SurveyQuestionProps) {
    const id = useId();

    if (question.kind === "scale") {
        return (
            <div className={styles.question}>
                <p id={id} className={styles.prompt} dir="auto">
                    {question.prompt}
                </p>
                <div role="group" aria-labelledby={id} className={styles.scale}>
                    {scaleValues(question).map(value => {
                        const chosen = picked.includes(value);

                        return (
                            <Button
                                key={value}
                                type="button"
                                size="small"
                                variant={chosen ? "primary" : "secondary"}
                                aria-pressed={chosen}
                                onClick={() => onPick(value)}
                            >
                                {value}
                            </Button>
                        );
                    })}
                </div>
            </div>
        );
    }

    const type = question.kind === "single_choice" ? "radio" : "checkbox";

    return (
        <fieldset className={styles.question}>
            <legend className={styles.prompt} dir="auto">
                {question.prompt}
            </legend>
            <div className={styles.choices}>
                {question.options.map((option, index) => (
                    <label key={`${index}:${option}`} className={styles.choice}>
                        <Input
                            type={type}
                            name={id}
                            className={styles.choiceInput}
                            checked={picked.includes(index)}
                            onChange={() => onPick(index)}
                        />
                        <span dir="auto">{option}</span>
                    </label>
                ))}
            </div>
        </fieldset>
    );
}

export function SurveyCard({ boardId, survey, canRespond, canManage }: SurveyCardProps) {
    const headingId = useId();
    const respond = useRespondToSurvey(boardId);
    const closer = useCloseSurvey(boardId);
    const remover = useDeleteSurvey(boardId);
    const [picks, setPicks] = useState<Record<string, number[]>>(() => ({ ...survey.my_answers }));
    const [confirming, setConfirming] = useState(false);

    const open = survey.status === "open";
    const answering = open && canRespond;
    const answered = Object.keys(survey.my_answers).length > 0;
    const failure = respond.error ?? closer.error ?? remover.error;

    function submit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();

        if (!answersComplete(survey, picks)) {
            return;
        }

        respond.mutate({ surveyId: survey.id, answers: surveyAnswers(survey, picks) });
    }

    return (
        <article aria-labelledby={headingId} className={styles.card}>
            <header className={styles.cardHeader}>
                <h3 id={headingId} className={styles.cardTitle} dir="auto">
                    {survey.title}
                </h3>
                <StatusTag tone={open ? "success" : "neutral"}>{open ? "Open" : "Closed"}</StatusTag>
            </header>
            <p className={styles.meta}>{respondentsLabel(survey.respondents)}</p>
            {answering && (
                <form className={styles.form} onSubmit={submit} aria-label={`Answer ${survey.title}`}>
                    {survey.questions.map(question => (
                        <SurveyQuestion
                            key={question.id}
                            question={question}
                            picked={picks[question.id] ?? []}
                            onPick={value =>
                                setPicks(current => ({
                                    ...current,
                                    [question.id]: pick(question, current[question.id] ?? [], value),
                                }))
                            }
                        />
                    ))}
                    {answered && (
                        <p className={styles.note}>You have answered. Submitting again replaces your answers.</p>
                    )}
                    <div className={styles.actions}>
                        <Button
                            type="submit"
                            variant="primary"
                            size="small"
                            disabled={respond.isPending || !answersComplete(survey, picks)}
                        >
                            Submit answers
                        </Button>
                    </div>
                </form>
            )}
            {open && !answering && (
                <>
                    <ol className={styles.prompts}>
                        {survey.questions.map(question => (
                            <li key={question.id} dir="auto">
                                {question.prompt}
                            </li>
                        ))}
                    </ol>
                    <p className={styles.note}>Results appear once the survey is closed.</p>
                </>
            )}
            {!open && survey.questions.map(question => <SurveyResults key={question.id} question={question} />)}
            {failure !== null && (
                <p role="alert" className={styles.error}>
                    {surveyRefusal(failure)}
                </p>
            )}
            {canManage && (
                <div className={styles.actions}>
                    {open && (
                        <Button
                            type="button"
                            size="small"
                            disabled={closer.isPending}
                            onClick={() => closer.mutate({ surveyId: survey.id })}
                        >
                            Close survey
                        </Button>
                    )}
                    <Button type="button" size="small" variant="danger" onClick={() => setConfirming(true)}>
                        Delete
                    </Button>
                </div>
            )}
            <ConfirmDialog
                open={confirming}
                title="Delete this survey?"
                body={`"${survey.title}" and every answer to it are removed for everyone on the board.`}
                confirmLabel="Delete survey"
                destructive
                busy={remover.isPending}
                onConfirm={() => remover.mutate({ surveyId: survey.id }, { onSettled: () => setConfirming(false) })}
                onCancel={() => setConfirming(false)}
            />
        </article>
    );
}
