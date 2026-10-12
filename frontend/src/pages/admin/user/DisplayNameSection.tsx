import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { Panel } from "../../../components/Panel/Panel";
import { Toast } from "../../../components/Toast/Toast";
import { DISPLAY_NAME_MAX_LENGTH } from "../../../domain/displayName";
import { useSetUserDisplayName } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import styles from "./UserSection.module.css";

interface DisplayNameSectionProps {
    user: AdminUserDetailResponse;
}

const NAME_MISSING = "Enter a display name.";

export function DisplayNameSection({ user }: DisplayNameSectionProps) {
    const rename = useSetUserDisplayName();
    const [name, setName] = useState(user.display_name);
    const [nameError, setNameError] = useState<string | null>(null);

    const summary = nameError ?? rename.error?.message ?? null;

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        const trimmed = name.trim();

        if (trimmed === "") {
            rename.reset();
            setNameError(NAME_MISSING);
            return;
        }

        setNameError(null);
        rename.mutate({ userId: user.id, display_name: trimmed });
    }

    return (
        <Panel title="Display name" headingLevel={3}>
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="Display name"
                    hint={`Up to ${DISPLAY_NAME_MAX_LENGTH} characters.`}
                    value={name}
                    onChange={e => setName(e.target.value)}
                    error={nameError}
                    maxLength={DISPLAY_NAME_MAX_LENGTH}
                    autoComplete="off"
                />
                <div>
                    <Button type="submit" variant="primary" disabled={rename.isPending}>
                        Save display name
                    </Button>
                </div>
            </form>
            {rename.isSuccess && (
                <Toast variant="success" onDismiss={rename.reset}>
                    Display name saved.
                </Toast>
            )}
        </Panel>
    );
}
