import { useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "../../components/Button/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../components/FormField/TextField";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { useDeleteAccount } from "../../hooks/mutations/auth";
import type { SessionUser } from "../../types/app";
import { errorCode } from "../../utils/errorMessage";
import styles from "./AccountSection.module.css";

interface DeleteAccountSectionProps {
    user: SessionUser;
}

const PASSWORD_MISSING = "Enter your password.";

export function DeleteAccountSection({ user }: DeleteAccountSectionProps) {
    const [open, setOpen] = useState(false);
    const [password, setPassword] = useState("");
    const [problem, setProblem] = useState<string | null>(null);
    const deletion = useDeleteAccount();
    const navigate = useNavigate();

    const failure = deletion.error?.message ?? null;
    const passwordError = problem ?? (errorCode(deletion.error) === "incorrect_password" ? failure : null);
    const summary = problem ?? failure;

    function submitDeletion() {
        if (password === "") {
            deletion.reset();
            setProblem(PASSWORD_MISSING);
            return;
        }

        setProblem(null);
        deletion.mutate(
            { password },
            {
                onSuccess: () => {
                    navigate("/", { replace: true });
                },
            },
        );
    }

    if (!user.has_password) {
        return (
            <SettingsRow
                title="Delete account"
                headingLevel={2}
                tone="danger"
                description="Set a password before you delete your account."
            />
        );
    }

    return (
        <SettingsRow
            title="Delete account"
            headingLevel={2}
            tone="danger"
            description="This deletes your account and signs you out. You cannot undo it."
        >
            <div className={styles.actions}>
                <Button
                    type="button"
                    variant="danger"
                    onClick={() => {
                        deletion.reset();
                        setProblem(null);
                        setOpen(true);
                    }}
                >
                    Delete account
                </Button>
            </div>
            <ConfirmDialog
                open={open}
                title="Delete your account?"
                body={
                    <form
                        className={styles.form}
                        onSubmit={e => {
                            e.preventDefault();
                            submitDeletion();
                        }}
                        noValidate
                    >
                        <p>Enter your password to delete your account. You cannot undo it.</p>
                        {summary !== null && <ErrorBanner message={summary} takeFocus />}
                        <TextField
                            label="Password"
                            type="password"
                            autoComplete="current-password"
                            error={passwordError}
                            value={password}
                            onChange={e => setPassword(e.target.value)}
                            required
                        />
                    </form>
                }
                confirmLabel="Delete account"
                destructive
                busy={deletion.isPending}
                onConfirm={submitDeletion}
                onCancel={() => {
                    setOpen(false);
                    setPassword("");
                    setProblem(null);
                    deletion.reset();
                }}
            />
        </SettingsRow>
    );
}
