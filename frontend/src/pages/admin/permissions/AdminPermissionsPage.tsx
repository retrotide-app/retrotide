import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { useAdminPermissions } from "../../../hooks/queries/admin";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminPermissionsPage.module.css";
import { RolePermissionsForm } from "./RolePermissionsForm";

export function AdminPermissionsPage() {
    const permissions = useAdminPermissions();
    const catalogue = permissions.data;

    return (
        <AdminPage title="Permissions" description="What each staff role is allowed to do on this site.">
            <p className={styles.intro}>
                Super admins and admins can do everything. Support is a limited staff role for helping users: choose
                what it can do here.
            </p>

            {permissions.error !== null && (
                <LoadFailure message={permissions.error.message} onRetry={permissions.refetch} />
            )}

            {catalogue === undefined && permissions.error === null && <p className="loading">Loading permissions...</p>}

            {catalogue !== undefined &&
                catalogue.roles.map(role => (
                    <RolePermissionsForm key={role.role} role={role} catalogue={catalogue.permissions} />
                ))}
        </AdminPage>
    );
}
