import { type SubmitEvent, useState } from "react";
import { useNavigate } from "react-router";
import { TemplatePicker } from "../../components/board/TemplatePicker";
import { Button } from "../../components/Button/Button";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormField } from "../../components/FormField/FormField";
import { TextField } from "../../components/FormField/TextField";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { Panel } from "../../components/Panel/Panel";
import { Select } from "../../components/Select/Select";
import { StepProgress } from "../../components/StepProgress/StepProgress";
import { TextArea } from "../../components/TextArea/TextArea";
import { ToggleSwitch } from "../../components/ToggleSwitch/ToggleSwitch";
import { BOARD_CONTEXT_MAX_LENGTH, BOARD_TITLE_MAX_LENGTH } from "../../domain/board/fields";
import { initialTeamChoice, NEW_BOARD_STEP_LABELS, NO_TEAM, newBoardSteps } from "../../domain/board/newBoard";
import { boardPath } from "../../domain/board/paths";
import { organisationHomePath } from "../../domain/organisations";
import {
    BOARD_ANONYMITY_REFUSALS,
    BOARD_CONTEXT_REFUSALS,
    BOARD_TEMPLATE_REFUSALS,
    BOARD_TITLE_REFUSALS,
    fieldRefusal,
} from "../../domain/refusals";
import { pickedTemplateOption, savedTemplateValue, templateFields, templateGroups } from "../../domain/savedTemplates";
import { useCreateBoard } from "../../hooks/mutations/boards";
import { useBoardTemplates } from "../../hooks/queries/boards";
import { useSavedTemplates } from "../../hooks/queries/savedTemplates";
import type { Anonymity, OrganisationResponse, TeamResponse } from "../../types/api";
import { ActionFailure } from "../org/shared/ActionFailure";
import sectionStyles from "../org/shared/OrgSection.module.css";

interface NewBoardWizardProps {
    organisation: OrganisationResponse;
    teams: readonly TeamResponse[];
    preset: TeamResponse | null;
    noTeamPreset: boolean;
    organisationBoards: boolean;
    presetTemplate?: string | null;
}

const TITLE_MISSING = "Enter a board title.";

const NO_TEAM_LABEL = "No team, everyone in the organisation";

const ANONYMITY_OPTIONS: readonly { value: Anonymity; label: string }[] = [
    { value: "named", label: "Named" },
    { value: "anonymous", label: "Anonymous" },
];

function anonymityFrom(value: string): Anonymity | undefined {
    return ANONYMITY_OPTIONS.find(option => option.value === value)?.value;
}

export function NewBoardWizard({
    organisation,
    teams,
    preset,
    noTeamPreset,
    organisationBoards,
    presetTemplate = null,
}: NewBoardWizardProps) {
    const navigate = useNavigate();
    const templates = useBoardTemplates();
    const savedTemplates = useSavedTemplates(organisation.id);
    const creation = useCreateBoard();
    const [stepIndex, setStepIndex] = useState(0);
    const [moved, setMoved] = useState(false);
    const [teamId, setTeamId] = useState(() => initialTeamChoice(teams, preset, organisationBoards));
    const [templateValue, setTemplateValue] = useState<string | null>(() =>
        presetTemplate === null ? null : savedTemplateValue(presetTemplate),
    );
    const [title, setTitle] = useState("");
    const [context, setContext] = useState("");
    const [anonymity, setAnonymity] = useState<Anonymity>("named");
    const [cardsShown, setCardsShown] = useState(false);
    const [problem, setProblem] = useState<string | null>(null);

    const steps = newBoardSteps(preset !== null || noTeamPreset);
    const step = steps[stepIndex];
    const team = teams.find(candidate => candidate.id === teamId) ?? null;

    const groups = templateGroups(savedTemplates.data?.templates ?? [], templates.data?.templates ?? []);
    const template = pickedTemplateOption(groups, templateValue);
    const templatesLoading =
        (templates.data === undefined && templates.error === null) ||
        (savedTemplates.data === undefined && savedTemplates.error === null);

    const titleError = problem ?? fieldRefusal(creation.error, BOARD_TITLE_REFUSALS);
    const templateError = fieldRefusal(creation.error, BOARD_TEMPLATE_REFUSALS);
    const contextError = fieldRefusal(creation.error, BOARD_CONTEXT_REFUSALS);
    const anonymityError = fieldRefusal(creation.error, BOARD_ANONYMITY_REFUSALS);

    function goTo(index: number) {
        setMoved(true);
        setStepIndex(index);
    }

    function handleNext(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (step === "template" && (templatesLoading || template === null)) {
            return;
        }

        goTo(stepIndex + 1);
    }

    function handleCreate(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        if (title.trim() === "") {
            creation.reset();
            setProblem(TITLE_MISSING);
            return;
        }

        if (template === null) {
            return;
        }

        setProblem(null);
        creation.mutate(
            {
                orgId: organisation.id,
                ...(team === null ? {} : { team_id: team.id }),
                title,
                ...templateFields(template.choice),
                context,
                anonymity,
                cards_shown: cardsShown,
            },
            {
                onSuccess: snapshot => {
                    navigate(boardPath(organisation.slug, snapshot.board_id));
                },
            },
        );
    }

    const back = stepIndex > 0 && (
        <Button type="button" onClick={() => goTo(stepIndex - 1)}>
            Back
        </Button>
    );

    const audience =
        team === null ? (
            <>
                everyone in <bdi>{organisation.name}</bdi>
            </>
        ) : (
            <bdi>{team.name}</bdi>
        );

    const cancel = <ButtonLink to={organisationHomePath(organisation.slug)}>Cancel</ButtonLink>;

    return (
        <div className={sectionStyles.stack}>
            <StepProgress label="Progress" steps={steps.map(each => NEW_BOARD_STEP_LABELS[each])} current={stepIndex} />
            {step === "team" && (
                <Panel
                    key="team"
                    title="Choose a team"
                    headingLevel={2}
                    description="A team board is shared with that team. A board with no team is open to everyone in the organisation."
                    takeFocus={moved}
                >
                    <form className={sectionStyles.form} onSubmit={handleNext} noValidate>
                        <FormField label="Team" layout="row">
                            {control => (
                                <Select
                                    {...control}
                                    value={team?.id ?? NO_TEAM}
                                    onChange={e => setTeamId(e.target.value)}
                                >
                                    {organisationBoards && <option value={NO_TEAM}>{NO_TEAM_LABEL}</option>}
                                    {teams.map(option => (
                                        <option key={option.id} value={option.id}>
                                            {option.name}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </FormField>
                        <div className={sectionStyles.actions}>
                            <Button type="submit" variant="primary">
                                Next
                            </Button>
                            {cancel}
                        </div>
                    </form>
                </Panel>
            )}
            {step === "template" && (
                <Panel
                    key="template"
                    title="Pick a template"
                    headingLevel={2}
                    description="Each template sets up the columns. You can rename, add and remove columns on the board later."
                    takeFocus={moved}
                >
                    {templates.error !== null && (
                        <LoadFailure message={templates.error.message} onRetry={templates.refetch} />
                    )}
                    {savedTemplates.error !== null && (
                        <LoadFailure message={savedTemplates.error.message} onRetry={savedTemplates.refetch} />
                    )}
                    {templatesLoading && <p className="loading">Loading templates...</p>}
                    {!templatesLoading && templates.data !== undefined && template === null && (
                        <p>No templates are available.</p>
                    )}
                    <form className={sectionStyles.form} onSubmit={handleNext} noValidate>
                        {!templatesLoading && template !== null && (
                            <TemplatePicker
                                legend="Template"
                                groups={groups}
                                selected={template.value}
                                onSelect={setTemplateValue}
                                error={templateError}
                            />
                        )}
                        <div className={sectionStyles.actions}>
                            {back}
                            <Button type="submit" variant="primary" disabled={templatesLoading || template === null}>
                                Next
                            </Button>
                            {cancel}
                        </div>
                    </form>
                </Panel>
            )}
            {step === "details" && template !== null && (
                <Panel
                    key="details"
                    title="Name your board"
                    headingLevel={2}
                    description={
                        <>
                            For {audience}, using <bdi>{template.name}</bdi>.
                        </>
                    }
                    takeFocus={moved}
                >
                    <form className={sectionStyles.form} onSubmit={handleCreate} noValidate>
                        {problem !== null && <ErrorBanner message={problem} takeFocus />}
                        {problem === null && creation.error !== null && (
                            <ActionFailure error={creation.error} takeFocus />
                        )}
                        <TextField
                            label="Board title"
                            hint={`Up to ${BOARD_TITLE_MAX_LENGTH} characters.`}
                            error={titleError}
                            value={title}
                            onChange={e => setTitle(e.target.value)}
                            maxLength={BOARD_TITLE_MAX_LENGTH}
                            autoComplete="off"
                            layout="row"
                            required
                        />
                        <FormField
                            label="Context"
                            hint={`Optional. What this retro covers, shown at the top of the board. Up to ${BOARD_CONTEXT_MAX_LENGTH} characters.`}
                            error={contextError}
                            layout="row"
                        >
                            {control => (
                                <TextArea
                                    {...control}
                                    value={context}
                                    onChange={e => setContext(e.target.value)}
                                    maxLength={BOARD_CONTEXT_MAX_LENGTH}
                                    rows={2}
                                />
                            )}
                        </FormField>
                        <FormField
                            label="Card authors"
                            hint="Named shows who wrote each card. Anonymous hides it from everyone on the board, facilitators included."
                            error={anonymityError}
                            layout="row"
                        >
                            {control => (
                                <Select
                                    {...control}
                                    value={anonymity}
                                    onChange={e => {
                                        const picked = anonymityFrom(e.target.value);

                                        if (picked !== undefined) {
                                            setAnonymity(picked);
                                        }
                                    }}
                                >
                                    {ANONYMITY_OPTIONS.map(option => (
                                        <option key={option.value} value={option.value}>
                                            {option.label}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </FormField>
                        <ToggleSwitch
                            label="Show cards as they are written"
                            description="When this is off, each person sees only their own cards. Everyone else's show as blurred placeholders."
                            enabled={cardsShown}
                            onChange={setCardsShown}
                        />
                        <div className={sectionStyles.actions}>
                            {back}
                            <Button type="submit" variant="primary" disabled={creation.isPending}>
                                Create board
                            </Button>
                            {cancel}
                        </div>
                    </form>
                </Panel>
            )}
        </div>
    );
}
