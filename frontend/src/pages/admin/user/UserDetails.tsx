import type { ReactNode } from "react";
import { Panel } from "../../../components/Panel/Panel";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { staffRoleLabel } from "../../../domain/staffRoles";
import type { AdminUserDetailResponse } from "../../../types/api";
import { formatExactDateTime } from "../../../utils/time";
import styles from "./UserDetails.module.css";

interface UserDetailsProps {
    user: AdminUserDetailResponse;
}

interface DetailProps {
    term: string;
    children: ReactNode;
}

function Detail({ term, children }: DetailProps) {
    return (
        <div className={styles.item}>
            <dt className={styles.term}>{term}</dt>
            <dd className={styles.value}>{children}</dd>
        </div>
    );
}

function present(value: string | undefined): value is string {
    return value !== undefined && value !== "";
}

export function UserDetails({ user }: UserDetailsProps) {
    const active = !user.banned && !user.locked;

    return (
        <Panel title="Details" headingLevel={3}>
            <dl className={styles.details}>
                <Detail term="Email">
                    <span className={styles.inline}>
                        <bdi>{user.email}</bdi>
                        {user.email_verified ? (
                            <StatusTag tone="success">Verified</StatusTag>
                        ) : (
                            <StatusTag tone="neutral">Unverified</StatusTag>
                        )}
                    </span>
                </Detail>
                <Detail term="Role">{staffRoleLabel(user.role)}</Detail>
                <Detail term="Status">
                    <span className={styles.inline}>
                        {user.banned && <StatusTag tone="danger">Banned</StatusTag>}
                        {user.locked && <StatusTag tone="warning">Locked</StatusTag>}
                        {active && <StatusTag tone="success">Active</StatusTag>}
                    </span>
                </Detail>
                <Detail term="Password">{user.has_password ? "Set" : "Not set"}</Detail>
                <Detail term="Joined">{formatExactDateTime(user.created_at)}</Detail>
                {user.banned && present(user.banned_at) && (
                    <Detail term="Banned on">{formatExactDateTime(user.banned_at)}</Detail>
                )}
                {user.banned && present(user.ban_reason) && (
                    <Detail term="Ban reason">
                        <span dir="auto">{user.ban_reason}</span>
                    </Detail>
                )}
                {user.locked && present(user.locked_at) && (
                    <Detail term="Locked on">{formatExactDateTime(user.locked_at)}</Detail>
                )}
                {user.locked && present(user.lock_reason) && (
                    <Detail term="Lock reason">
                        <span dir="auto">{user.lock_reason}</span>
                    </Detail>
                )}
                {present(user.last_seen_ip) && (
                    <Detail term="Last seen from">
                        <span className={styles.address}>{user.last_seen_ip}</span>
                        {present(user.last_seen_ip_at) && ` on ${formatExactDateTime(user.last_seen_ip_at)}`}
                    </Detail>
                )}
            </dl>
        </Panel>
    );
}
