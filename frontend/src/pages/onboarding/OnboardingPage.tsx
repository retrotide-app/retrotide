import { type SubmitEvent, useState } from "react";
import { Navigate, useNavigate } from "react-router";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../components/FormField/TextField";
import { AuthLayout } from "../../components/layout/AuthLayout/AuthLayout";
import {
    INVITE_PATH,
    ORGANISATION_NAME_MAX_LENGTH,
    organisationHomePath,
    TEAM_NAME_MAX_LENGTH,
} from "../../domain/organisations";
import { fieldRefusal, ORGANISATION_NAME_REFUSALS, TEAM_NAME_REFUSALS } from "../../domain/refusals";
import { useCreateWorkspace } from "../../hooks/mutations/organisations";
import { useAuth } from "../../hooks/useAuth";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import { useHasPendingInvite } from "../../hooks/useInviteToken";
import { usePageTitle } from "../../hooks/usePageTitle";
import { errorMessage } from "../../utils/errorMessage";
import authStyles from "../auth/AuthForm.module.css";
import { SetPasswordOffer } from "./SetPasswordOffer";
import styles from "./OnboardingPage.module.css";

interface WorkspaceProblems {
    name?: string;
    teamName?: string;
}

const TITLE = "Create your workspace";

const CREATE_FAILED = "Creating your workspace failed. Try again.";

function findProblems(name: string, teamName: string): WorkspaceProblems {
    const problems: WorkspaceProblems = {};

    if (name.trim() === "") {
        problems.name = "Enter an organisation name.";
    }

    if (teamName.trim() === "") {
        problems.teamName = "Enter a team name.";
    }

    return problems;
}

export function OnboardingPage() {
    usePageTitle(TITLE);

    const { user } = useAuth();
    const { organisations, loading, selectOrganisation } = useCurrentOrg();
    const hasPendingInvite = useHasPendingInvite();
    const createWorkspace = useCreateWorkspace();
    const navigate = useNavigate();
    const [name, setName] = useState("");
    const [teamName, setTeamName] = useState("");
    const [problems, setProblems] = useState<WorkspaceProblems>({});

    if (hasPendingInvite && !loading && organisations.length === 0) {
        return <Navigate to={INVITE_PATH} replace />;
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        const found = findProblems(name, teamName);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            createWorkspace.reset();
            return;
        }

        createWorkspace.mutate(
            { name: name.trim(), team_name: teamName.trim() },
            {
                onSuccess: workspace => {
                    selectOrganisation(workspace.organisation.id);
                    navigate(organisationHomePath(workspace.organisation.slug), { replace: true });
                },
            },
        );
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (createWorkspace.error !== null) {
            return errorMessage(createWorkspace.error, CREATE_FAILED);
        }

        return null;
    }

    const message = summary();
    const nameError = problems.name ?? fieldRefusal(createWorkspace.error, ORGANISATION_NAME_REFUSALS);
    const teamNameError = problems.teamName ?? fieldRefusal(createWorkspace.error, TEAM_NAME_REFUSALS);

    return (
        <AuthLayout title={TITLE}>
            <p className={authStyles.lead}>Name your organisation and its first team. You can rename both later.</p>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <form className={authStyles.form} onSubmit={handleSubmit} aria-label={TITLE} noValidate>
                <TextField
                    label="Organisation name"
                    hint={`Up to ${ORGANISATION_NAME_MAX_LENGTH} characters.`}
                    type="text"
                    name="organisation"
                    autoComplete="organization"
                    maxLength={ORGANISATION_NAME_MAX_LENGTH}
                    value={name}
                    onChange={event => setName(event.target.value)}
                    error={nameError}
                />
                <TextField
                    label="Team name"
                    hint={`Your first team. Up to ${TEAM_NAME_MAX_LENGTH} characters.`}
                    type="text"
                    name="team"
                    autoComplete="off"
                    maxLength={TEAM_NAME_MAX_LENGTH}
                    value={teamName}
                    onChange={event => setTeamName(event.target.value)}
                    error={teamNameError}
                />
                <Button
                    type="submit"
                    variant="primary"
                    className={authStyles.wide}
                    disabled={createWorkspace.isPending}
                >
                    Create workspace
                </Button>
            </form>
            {user !== null && !user.has_password && (
                <div className={styles.offer}>
                    <SetPasswordOffer email={user.email} />
                </div>
            )}
        </AuthLayout>
    );
}
