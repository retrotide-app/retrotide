import { useParams } from "react-router";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { TextLink } from "../../../components/TextLink/TextLink";
import { staffActionBlock } from "../../../domain/staffRoles";
import { useAdminUser } from "../../../hooks/queries/admin";
import { useAuth } from "../../../hooks/useAuth";
import type { AdminUserDetailResponse } from "../../../types/api";
import { AdminPage } from "../shared/AdminPage";
import { BanSection } from "./BanSection";
import { DeleteUserSection } from "./DeleteUserSection";
import { DisplayNameSection } from "./DisplayNameSection";
import { EmailSection } from "./EmailSection";
import { LockSection } from "./LockSection";
import { PasswordSection } from "./PasswordSection";
import { RoleSection } from "./RoleSection";
import { SignOutSection } from "./SignOutSection";
import { UserAuditLogSection } from "./UserAuditLogSection";
import { UserDetails } from "./UserDetails";
import styles from "./AdminUserPage.module.css";

interface UserActionsProps {
    user: AdminUserDetailResponse;
}

function UserActions({ user }: UserActionsProps) {
    const { user: viewer, permissions } = useAuth();
    const block = staffActionBlock(viewer?.id ?? null, permissions, user);

    if (block === "self") {
        return (
            <p className={styles.note}>
                This is your own account. Change it from your <TextLink to="/account">account settings</TextLink>.
            </p>
        );
    }

    if (block === "super_admin") {
        return <p className={styles.note}>Nobody can change a super admin&apos;s account here.</p>;
    }

    if (block === "outranked") {
        return <p className={styles.note}>Only an admin can change another staff member&apos;s account.</p>;
    }

    const canChangeEmail = permissions.includes("manage_user_email");
    const canVerifyEmail = permissions.includes("set_email_verified");
    const canRestrict = permissions.includes("ban_user");

    return (
        <>
            {permissions.includes("manage_user_account") && <DisplayNameSection user={user} />}
            {(canChangeEmail || canVerifyEmail) && (
                <EmailSection user={user} canChange={canChangeEmail} canVerify={canVerifyEmail} />
            )}
            {permissions.includes("manage_roles") && <RoleSection user={user} />}
            {canRestrict && <BanSection user={user} />}
            {canRestrict && <LockSection user={user} />}
            {canRestrict && <SignOutSection user={user} />}
            {permissions.includes("reset_password") && <PasswordSection user={user} />}
            {permissions.includes("delete_any_user") && <DeleteUserSection user={user} />}
        </>
    );
}

export function AdminUserPage() {
    const { userId = "" } = useParams();
    const { permissions } = useAuth();
    const record = useAdminUser(userId);
    const user = record.data;

    const failure = record.error !== null && <LoadFailure message={record.error.message} onRetry={record.refetch} />;
    const back = (
        <p>
            <TextLink to="/admin/users">Back to users</TextLink>
        </p>
    );

    if (user === undefined) {
        return (
            <AdminPage title="User">
                {back}
                {failure}
                {record.error === null && <p className="loading">Loading user...</p>}
            </AdminPage>
        );
    }

    return (
        <AdminPage title={user.display_name} description="This account's details, staff role, sign-in and access.">
            {back}
            {failure}
            <div className={styles.sections} key={user.id}>
                <UserDetails user={user} />
                <UserActions user={user} />
                {permissions.includes("view_audit_log") && <UserAuditLogSection user={user} />}
            </div>
        </AdminPage>
    );
}
