import { useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { Panel } from "../../../components/Panel/Panel";
import { useDeleteUser } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import styles from "./UserSection.module.css";

interface DeleteUserSectionProps {
    user: AdminUserDetailResponse;
}

export function DeleteUserSection({ user }: DeleteUserSectionProps) {
    const deleteUser = useDeleteUser();
    const navigate = useNavigate();
    const [confirming, setConfirming] = useState(false);

    function openConfirm() {
        deleteUser.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        deleteUser.reset();
        setConfirming(false);
    }

    function handleDelete() {
        deleteUser.mutate(
            { userId: user.id },
            {
                onSuccess: () => {
                    setConfirming(false);
                    navigate("/admin/users", { replace: true });
                },
            },
        );
    }

    const dialogBody = (
        <div className={styles.form}>
            <p>
                <bdi>{user.display_name}</bdi>&apos;s account and everything in it is deleted. This cannot be undone.
            </p>
            {deleteUser.error !== null && <ErrorBanner message={deleteUser.error.message} takeFocus />}
        </div>
    );

    return (
        <Panel
            tone="danger"
            title="Delete user"
            headingLevel={3}
            description="Deletes the account for good. This cannot be undone."
        >
            <div>
                <Button type="button" variant="danger" onClick={openConfirm}>
                    Delete user
                </Button>
            </div>
            <ConfirmDialog
                open={confirming}
                title="Delete user"
                body={dialogBody}
                confirmLabel="Delete user"
                destructive
                busy={deleteUser.isPending}
                onConfirm={handleDelete}
                onCancel={closeConfirm}
            />
        </Panel>
    );
}
