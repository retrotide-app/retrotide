import { useState } from "react";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../../components/FormDialog/FormDialog";
import { TextField } from "../../../components/FormField/TextField";
import { TEAM_NAME_MAX_LENGTH } from "../../../domain/organisations";
import { fieldRefusal, TEAM_NAME_REFUSALS } from "../../../domain/refusals";
import { ActionFailure } from "./ActionFailure";

interface TeamNameDialogProps {
    open: boolean;
    title: string;
    submitLabel: string;
    initialName?: string;
    busy: boolean;
    error: Error | null;
    onSubmit: (name: string) => void;
    onClose: () => void;
}

type TeamNameFormProps = Omit<TeamNameDialogProps, "open">;

const NAME_MISSING = "Enter a team name.";

function TeamNameForm({ title, submitLabel, initialName = "", busy, error, onSubmit, onClose }: TeamNameFormProps) {
    const [name, setName] = useState(initialName);
    const [problem, setProblem] = useState<string | null>(null);

    const fieldError = problem ?? fieldRefusal(error, TEAM_NAME_REFUSALS);

    function handleSubmit() {
        if (name.trim() === "") {
            setProblem(NAME_MISSING);
            return;
        }

        setProblem(null);
        onSubmit(name);
    }

    return (
        <FormDialog open title={title} submitLabel={submitLabel} busy={busy} onSubmit={handleSubmit} onCancel={onClose}>
            {problem !== null && <ErrorBanner message={problem} takeFocus />}
            {problem === null && error !== null && <ActionFailure error={error} takeFocus />}
            <TextField
                label="Team name"
                hint={`Up to ${TEAM_NAME_MAX_LENGTH} characters.`}
                error={fieldError}
                value={name}
                onChange={e => setName(e.target.value)}
                maxLength={TEAM_NAME_MAX_LENGTH}
                autoComplete="off"
                required
            />
        </FormDialog>
    );
}

export function TeamNameDialog({ open, ...form }: TeamNameDialogProps) {
    if (!open) {
        return null;
    }

    return <TeamNameForm {...form} />;
}
