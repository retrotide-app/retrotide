import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { Panel } from "../../../components/Panel/Panel";
import { Toast } from "../../../components/Toast/Toast";
import { ToggleSwitch } from "../../../components/ToggleSwitch/ToggleSwitch";
import { samePermissions, withPermission } from "../../../domain/staffRoles";
import { useUpdateRolePermissions } from "../../../hooks/mutations/admin";
import type { PermissionCatalogueItem, RolePermissionsItem } from "../../../types/api";
import styles from "./AdminPermissionsPage.module.css";

interface RolePermissionsFormProps {
    role: RolePermissionsItem;
    catalogue: readonly PermissionCatalogueItem[];
}

export function RolePermissionsForm({ role, catalogue }: RolePermissionsFormProps) {
    const update = useUpdateRolePermissions();
    const [draft, setDraft] = useState<string[] | null>(null);
    const granted = draft ?? role.permissions;
    const changed = !samePermissions(granted, role.permissions);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        update.mutate(
            { role: role.role, permissions: granted },
            {
                onSuccess: () => {
                    setDraft(null);
                },
            },
        );
    }

    return (
        <Panel title={role.label} headingLevel={3} description="Changes apply once you save them.">
            <form className={styles.form} onSubmit={handleSubmit}>
                {update.error !== null && <ErrorBanner message={update.error.message} takeFocus />}
                <div role="group" aria-label={`${role.label} permissions`} className={styles.toggles}>
                    {catalogue.map(item => (
                        <ToggleSwitch
                            key={item.permission}
                            label={item.label}
                            enabled={granted.includes(item.permission)}
                            onChange={on => setDraft(withPermission(granted, item.permission, on))}
                            disabled={update.isPending}
                        />
                    ))}
                </div>
                <div>
                    <Button type="submit" variant="primary" disabled={!changed || update.isPending}>
                        Save permissions
                    </Button>
                </div>
            </form>
            {update.isSuccess && (
                <Toast variant="success" onDismiss={update.reset}>
                    Permissions saved.
                </Toast>
            )}
        </Panel>
    );
}
