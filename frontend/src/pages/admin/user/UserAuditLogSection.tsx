import { useState } from "react";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Pagination } from "../../../components/Pagination/Pagination";
import { Panel } from "../../../components/Panel/Panel";
import { useAdminUserAuditLog } from "../../../hooks/queries/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import { AuditLogTable } from "../shared/AuditLogTable";
import styles from "./UserSection.module.css";

const PAGE_SIZE = 50;

interface UserAuditLogSectionProps {
    user: AdminUserDetailResponse;
}

export function UserAuditLogSection({ user }: UserAuditLogSectionProps) {
    const [offset, setOffset] = useState(0);
    const log = useAdminUserAuditLog(user.id, { limit: PAGE_SIZE, offset });
    const listing = log.data;

    return (
        <Panel title="Audit log" headingLevel={3}>
            {log.error !== null && <LoadFailure message={log.error.message} onRetry={log.refetch} />}
            {listing === undefined && log.error === null && <p className="loading">Loading the audit log...</p>}
            {listing !== undefined && listing.total === 0 && (
                <p className={styles.note}>Nothing has been recorded about this user yet.</p>
            )}
            {listing !== undefined && listing.entries.length > 0 && (
                <AuditLogTable
                    caption={`Audit log for ${user.display_name}`}
                    entries={listing.entries}
                    showSubject={false}
                    linkSubjects={false}
                />
            )}
            {listing !== undefined && (
                <Pagination
                    label="Audit log pages"
                    offset={listing.offset}
                    limit={listing.limit}
                    total={listing.total}
                    onChange={setOffset}
                />
            )}
        </Panel>
    );
}
