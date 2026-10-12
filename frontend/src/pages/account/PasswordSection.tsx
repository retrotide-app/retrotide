import { useState } from "react";
import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../components/FormDialog/FormDialog";
import { TextField } from "../../components/FormField/TextField";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { SetPasswordForm } from "../../components/SetPasswordForm/SetPasswordForm";
import { useChangePassword } from "../../hooks/mutations/auth";
import type { SessionUser } from "../../types/app";
import { errorCode } from "../../utils/errorMessage";
import styles from "./AccountSection.module.css";

interface PasswordSectionProps {
    user: SessionUser;
    onPasswordChanged: () => void;
}

interface ChangePasswordProps {
    onChanged: () => void;
}

interface ChangePasswordProblems {
    currentPassword?: string;
    newPassword?: string;
}

function findProblems(currentPassword: string, newPassword: string): ChangePasswordProblems {
    const problems: ChangePasswordProblems = {};

    if (currentPassword === "") {
        problems.currentPassword = "Enter your current password.";
    }

    if (newPassword === "") {
        problems.newPassword = "Enter a new password.";
    }

    return problems;
}

function ChangePassword({ onChanged }: ChangePasswordProps) {
    const [open, setOpen] = useState(false);
    const [currentPassword, setCurrentPassword] = useState("");
    const [newPassword, setNewPassword] = useState("");
    const [problems, setProblems] = useState<ChangePasswordProblems>({});
    const change = useChangePassword();

    const failure = change.error?.message ?? null;
    const code = errorCode(change.error);
    const listed = Object.values(problems).join(" ");
    const summary = listed === "" ? failure : listed;

    function clearFields() {
        setCurrentPassword("");
        setNewPassword("");
        setProblems({});
    }

    function handleSubmit() {
        const found = findProblems(currentPassword, newPassword);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            change.reset();
            return;
        }

        change.mutate(
            { current_password: currentPassword, new_password: newPassword },
            {
                onSuccess: () => {
                    setOpen(false);
                    clearFields();
                    onChanged();
                },
            },
        );
    }

    return (
        <>
            <div className={styles.actions}>
                <Button
                    type="button"
                    onClick={() => {
                        change.reset();
                        clearFields();
                        setOpen(true);
                    }}
                >
                    Change password
                </Button>
            </div>
            {change.isSuccess && !open && (
                <Banner tone="success" role="status">
                    Password changed. You are signed out on every other device.
                </Banner>
            )}
            <FormDialog
                open={open}
                title="Change password"
                submitLabel="Save new password"
                busy={change.isPending}
                onSubmit={handleSubmit}
                onCancel={() => {
                    setOpen(false);
                    clearFields();
                    change.reset();
                }}
            >
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="Current password"
                    type="password"
                    autoComplete="current-password"
                    error={problems.currentPassword ?? (code === "incorrect_password" ? failure : null)}
                    value={currentPassword}
                    onChange={e => setCurrentPassword(e.target.value)}
                    required
                />
                <TextField
                    label="New password"
                    type="password"
                    autoComplete="new-password"
                    error={problems.newPassword ?? (code === "password_too_short" ? failure : null)}
                    value={newPassword}
                    onChange={e => setNewPassword(e.target.value)}
                    required
                />
            </FormDialog>
        </>
    );
}

export function PasswordSection({ user, onPasswordChanged }: PasswordSectionProps) {
    if (user.has_password) {
        return (
            <SettingsRow
                title="Password"
                headingLevel={2}
                description="Changing it signs you out on every other device."
            >
                <ChangePassword onChanged={onPasswordChanged} />
            </SettingsRow>
        );
    }

    const description = (
        <>
            Your account has no password. We will email a link to <bdi className={styles.address}>{user.email}</bdi>{" "}
            that lets you set one.
        </>
    );

    return (
        <SettingsRow title="Password" headingLevel={2} description={description}>
            <SetPasswordForm email={user.email} />
        </SettingsRow>
    );
}
