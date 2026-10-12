import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { Panel } from "../../../components/Panel/Panel";
import { Toast } from "../../../components/Toast/Toast";
import { useForceLogoutUser } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import styles from "./UserSection.module.css";

interface SignOutSectionProps {
    user: AdminUserDetailResponse;
}

export function SignOutSection({ user }: SignOutSectionProps) {
    const signOut = useForceLogoutUser();
    const [confirming, setConfirming] = useState(false);

    function openConfirm() {
        signOut.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        signOut.reset();
        setConfirming(false);
    }

    function handleSignOut() {
        signOut.mutate(
            { userId: user.id },
            {
                onSuccess: () => {
                    setConfirming(false);
                },
            },
        );
    }

    const dialogBody = (
        <div className={styles.form}>
            <p>
                <bdi>{user.display_name}</bdi> is signed out on every device. Their password stays the same.
            </p>
            {signOut.error !== null && <ErrorBanner message={signOut.error.message} takeFocus />}
        </div>
    );

    return (
        <Panel
            title="Sessions"
            headingLevel={3}
            description="Signs the user out on every device without changing their password."
        >
            <div>
                <Button type="button" variant="danger" onClick={openConfirm}>
                    Sign out everywhere
                </Button>
            </div>
            <ConfirmDialog
                open={confirming}
                title="Sign out everywhere"
                body={dialogBody}
                confirmLabel="Sign out everywhere"
                destructive
                busy={signOut.isPending}
                onConfirm={handleSignOut}
                onCancel={closeConfirm}
            />
            {signOut.isSuccess && (
                <Toast variant="success" onDismiss={signOut.reset}>
                    Signed out everywhere.
                </Toast>
            )}
        </Panel>
    );
}
