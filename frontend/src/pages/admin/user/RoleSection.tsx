import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { FormField } from "../../../components/FormField/FormField";
import { Panel } from "../../../components/Panel/Panel";
import { Select } from "../../../components/Select/Select";
import { Toast } from "../../../components/Toast/Toast";
import {
    GRANTABLE_STAFF_ROLES,
    isGrantableStaffRole,
    STAFF_ROLE_LABELS,
    staffRoleLabel,
} from "../../../domain/staffRoles";
import { useRemoveUserRole, useSetUserRole } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse, StaffRole } from "../../../types/api";
import styles from "./UserSection.module.css";

interface RoleSectionProps {
    user: AdminUserDetailResponse;
}

export function RoleSection({ user }: RoleSectionProps) {
    const setRole = useSetUserRole();
    const removeRole = useRemoveUserRole();
    const [role, setChosenRole] = useState<StaffRole>(isGrantableStaffRole(user.role) ? user.role : "support");
    const [confirming, setConfirming] = useState(false);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        setRole.mutate({ userId: user.id, role });
    }

    function openConfirm() {
        removeRole.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        removeRole.reset();
        setConfirming(false);
    }

    function handleRemove() {
        if (user.role === undefined) {
            return;
        }

        removeRole.mutate(
            { userId: user.id, role: user.role },
            {
                onSuccess: () => {
                    setConfirming(false);
                },
            },
        );
    }

    const currentLabel = staffRoleLabel(user.role);
    const setRoleLabel = setRole.variables === undefined ? "" : STAFF_ROLE_LABELS[setRole.variables.role];

    const dialogBody = (
        <div className={styles.form}>
            <p>
                <bdi>{user.display_name}</bdi> loses the {currentLabel} role and every permission it gives.
            </p>
            {removeRole.error !== null && <ErrorBanner message={removeRole.error.message} takeFocus />}
        </div>
    );

    return (
        <Panel title="Role" headingLevel={3} description={`Current role: ${currentLabel}.`}>
            <form className={styles.form} onSubmit={handleSubmit}>
                {setRole.error !== null && <ErrorBanner message={setRole.error.message} takeFocus />}
                <FormField label="Role">
                    {control => (
                        <Select
                            {...control}
                            value={role}
                            onChange={e => {
                                if (isGrantableStaffRole(e.target.value)) {
                                    setChosenRole(e.target.value);
                                }
                            }}
                        >
                            {GRANTABLE_STAFF_ROLES.map(option => (
                                <option key={option} value={option}>
                                    {STAFF_ROLE_LABELS[option]}
                                </option>
                            ))}
                        </Select>
                    )}
                </FormField>
                <div className={styles.actions}>
                    <Button type="submit" variant="primary" disabled={setRole.isPending || role === user.role}>
                        Set role
                    </Button>
                    {user.role !== undefined && (
                        <Button type="button" variant="danger" onClick={openConfirm}>
                            Remove role
                        </Button>
                    )}
                </div>
            </form>
            <ConfirmDialog
                open={confirming}
                title="Remove role"
                body={dialogBody}
                confirmLabel="Remove role"
                destructive
                busy={removeRole.isPending}
                onConfirm={handleRemove}
                onCancel={closeConfirm}
            />
            {setRole.isSuccess && (
                <Toast variant="success" onDismiss={setRole.reset}>
                    Role set to {setRoleLabel}.
                </Toast>
            )}
            {removeRole.isSuccess && (
                <Toast variant="success" onDismiss={removeRole.reset}>
                    Role removed.
                </Toast>
            )}
        </Panel>
    );
}
