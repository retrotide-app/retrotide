import { useState } from "react";
import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../components/FormDialog/FormDialog";
import { TextField } from "../../components/FormField/TextField";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { DISPLAY_NAME_MAX_LENGTH } from "../../domain/displayName";
import { useUpdateProfile } from "../../hooks/mutations/auth";
import type { SessionUser } from "../../types/app";
import { errorStatus } from "../../utils/errorMessage";
import styles from "./AccountSection.module.css";

interface ProfileSectionProps {
    user: SessionUser;
}

const NAME_MISSING = "Enter a display name.";

export function ProfileSection({ user }: ProfileSectionProps) {
    const [open, setOpen] = useState(false);
    const [displayName, setDisplayName] = useState(user.display_name);
    const [problem, setProblem] = useState<string | null>(null);
    const update = useUpdateProfile();

    const failure = update.error?.message ?? null;
    const fieldError = problem ?? (errorStatus(update.error) === 400 ? failure : null);
    const summary = problem ?? failure;

    function handleSubmit() {
        if (displayName.trim() === "") {
            update.reset();
            setProblem(NAME_MISSING);
            return;
        }

        setProblem(null);
        update.mutate({ display_name: displayName }, { onSuccess: () => setOpen(false) });
    }

    const description = (
        <>
            Your display name is <bdi className={styles.address}>{user.display_name}</bdi>. It shows on boards and to
            the people in your organisations.
        </>
    );

    return (
        <SettingsRow title="Profile" headingLevel={2} description={description}>
            <div className={styles.actions}>
                <Button
                    type="button"
                    onClick={() => {
                        update.reset();
                        setProblem(null);
                        setDisplayName(user.display_name);
                        setOpen(true);
                    }}
                >
                    Change display name
                </Button>
            </div>
            {update.isSuccess && !open && (
                <Banner tone="success" role="status">
                    Display name saved.
                </Banner>
            )}
            <FormDialog
                open={open}
                title="Change display name"
                submitLabel="Save display name"
                busy={update.isPending}
                onSubmit={handleSubmit}
                onCancel={() => {
                    setOpen(false);
                    setProblem(null);
                    update.reset();
                }}
            >
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="Display name"
                    hint={`Up to ${DISPLAY_NAME_MAX_LENGTH} characters.`}
                    error={fieldError}
                    value={displayName}
                    onChange={e => setDisplayName(e.target.value)}
                    maxLength={DISPLAY_NAME_MAX_LENGTH}
                    autoComplete="nickname"
                    required
                />
            </FormDialog>
        </SettingsRow>
    );
}
