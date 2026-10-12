import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { Modal } from "../../../components/Modal/Modal";
import { Panel } from "../../../components/Panel/Panel";
import { useResetUserPassword } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import styles from "./UserSection.module.css";

interface PasswordSectionProps {
    user: AdminUserDetailResponse;
}

export function PasswordSection({ user }: PasswordSectionProps) {
    const resetPassword = useResetUserPassword();
    const [confirming, setConfirming] = useState(false);
    const [password, setPassword] = useState<string | null>(null);

    function openConfirm() {
        resetPassword.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        resetPassword.reset();
        setConfirming(false);
    }

    function handleReset() {
        resetPassword.mutate(
            { userId: user.id },
            {
                onSuccess: result => {
                    setConfirming(false);
                    setPassword(result.password);
                },
            },
        );
    }

    function forgetPassword() {
        setPassword(null);
        resetPassword.reset();
    }

    const dialogBody = (
        <div className={styles.form}>
            <p>
                <bdi>{user.display_name}</bdi>&apos;s password is replaced with a new one and they are signed out
                everywhere. You see the new password once.
            </p>
            {resetPassword.error !== null && <ErrorBanner message={resetPassword.error.message} takeFocus />}
        </div>
    );

    return (
        <Panel
            title="Password"
            headingLevel={3}
            description="Replaces the password with a new random one and signs the user out everywhere."
        >
            <div>
                <Button type="button" variant="danger" onClick={openConfirm}>
                    Reset password
                </Button>
            </div>
            <ConfirmDialog
                open={confirming}
                title="Reset password"
                body={dialogBody}
                confirmLabel="Reset password"
                destructive
                busy={resetPassword.isPending}
                onConfirm={handleReset}
                onCancel={closeConfirm}
            />
            <Modal isOpen={password !== null} onClose={forgetPassword} title="New password" closeOnOverlayClick={false}>
                <div className={styles.form}>
                    <p>
                        Give this password to <bdi>{user.display_name}</bdi>. It is not shown again.
                    </p>
                    <TextField
                        label="New password"
                        value={password ?? ""}
                        readOnly
                        autoComplete="off"
                        spellCheck={false}
                        onFocus={e => e.currentTarget.select()}
                    />
                    <div>
                        <Button type="button" variant="primary" onClick={forgetPassword}>
                            Done
                        </Button>
                    </div>
                </div>
            </Modal>
        </Panel>
    );
}
