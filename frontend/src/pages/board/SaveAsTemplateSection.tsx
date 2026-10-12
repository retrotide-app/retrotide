import { type SubmitEvent, useState } from "react";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormField } from "../../components/FormField/FormField";
import { TextField } from "../../components/FormField/TextField";
import { Panel } from "../../components/Panel/Panel";
import { Select } from "../../components/Select/Select";
import { TextArea } from "../../components/TextArea/TextArea";
import { TextLink } from "../../components/TextLink/TextLink";
import { Toast } from "../../components/Toast/Toast";
import {
    organisationTemplatesPath,
    SAVED_TEMPLATE_DESCRIPTION_MAX_LENGTH,
    SAVED_TEMPLATE_NAME_MAX_LENGTH,
    SAVED_TEMPLATE_SCOPE_LABELS,
    savedTemplateRefusal,
} from "../../domain/savedTemplates";
import { useSaveBoardAsTemplate } from "../../hooks/mutations/savedTemplates";
import type { BoardView, OrganisationResponse, SavedTemplateResponse, SavedTemplateScope } from "../../types/api";
import { ActionFailure } from "../org/shared/ActionFailure";
import sectionStyles from "../org/shared/OrgSection.module.css";

interface SaveAsTemplateSectionProps {
    organisation: OrganisationResponse;
    board: BoardView;
}

const NAME_MISSING = "Enter a template name.";

const SCOPES: readonly SavedTemplateScope[] = ["personal", "team"];

function scopeFrom(value: string): SavedTemplateScope | undefined {
    return SCOPES.find(scope => scope === value);
}

export function SaveAsTemplateSection({ organisation, board }: SaveAsTemplateSectionProps) {
    const save = useSaveBoardAsTemplate();
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [scope, setScope] = useState<SavedTemplateScope>("personal");
    const [problem, setProblem] = useState<string | null>(null);
    const [saved, setSaved] = useState<SavedTemplateResponse | null>(null);

    const teamBoard = board.team_id !== undefined && board.team_id !== "";

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        setSaved(null);

        if (name.trim() === "") {
            save.reset();
            setProblem(NAME_MISSING);
            return;
        }

        setProblem(null);
        save.mutate(
            { orgId: organisation.id, boardId: board.id, name, description, scope: teamBoard ? scope : "personal" },
            {
                onSuccess: template => {
                    setSaved(template);
                    setName("");
                    setDescription("");
                },
            },
        );
    }

    return (
        <Panel
            title="Save as template"
            headingLevel={2}
            description="Copy this board's columns into a template for new boards. Cards, votes and comments stay on the board."
        >
            <form className={sectionStyles.form} onSubmit={handleSubmit} aria-label="Save as template" noValidate>
                {problem !== null && <ErrorBanner message={problem} takeFocus />}
                {problem === null && save.error !== null && <ActionFailure error={save.error} takeFocus />}
                {saved !== null && (
                    <Toast variant="success">
                        Saved <bdi>{saved.name}</bdi>.{" "}
                        <TextLink to={organisationTemplatesPath(organisation.slug)}>See your templates</TextLink>
                    </Toast>
                )}
                <TextField
                    label="Template name"
                    hint={`Up to ${SAVED_TEMPLATE_NAME_MAX_LENGTH} characters.`}
                    error={problem ?? savedTemplateRefusal(save.error, "name")}
                    value={name}
                    onChange={e => setName(e.target.value)}
                    maxLength={SAVED_TEMPLATE_NAME_MAX_LENGTH}
                    autoComplete="off"
                    layout="row"
                    required
                />
                <FormField
                    label="Template description"
                    hint={`Optional. Up to ${SAVED_TEMPLATE_DESCRIPTION_MAX_LENGTH} characters.`}
                    error={savedTemplateRefusal(save.error, "description")}
                    layout="row"
                >
                    {control => (
                        <TextArea
                            {...control}
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            maxLength={SAVED_TEMPLATE_DESCRIPTION_MAX_LENGTH}
                            rows={2}
                        />
                    )}
                </FormField>
                {teamBoard && (
                    <FormField
                        label="Who can use it"
                        hint="A team template is offered to everyone on the board's team."
                        error={savedTemplateRefusal(save.error, "scope")}
                        layout="row"
                    >
                        {control => (
                            <Select
                                {...control}
                                value={scope}
                                onChange={e => {
                                    const picked = scopeFrom(e.target.value);

                                    if (picked !== undefined) {
                                        setScope(picked);
                                    }
                                }}
                            >
                                {SCOPES.map(option => (
                                    <option key={option} value={option}>
                                        {SAVED_TEMPLATE_SCOPE_LABELS[option]}
                                    </option>
                                ))}
                            </Select>
                        )}
                    </FormField>
                )}
                <div className={sectionStyles.actions}>
                    <Button type="submit" variant="primary" disabled={save.isPending}>
                        Save as template
                    </Button>
                </div>
            </form>
        </Panel>
    );
}
