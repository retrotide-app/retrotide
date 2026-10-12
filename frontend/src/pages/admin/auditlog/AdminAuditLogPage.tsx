import { useState } from "react";
import { FormField } from "../../../components/FormField/FormField";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Pagination } from "../../../components/Pagination/Pagination";
import { Select } from "../../../components/Select/Select";
import { AUDIT_ACTION_LABELS, isAuditAction, SITE_AUDIT_ACTIONS } from "../../../domain/auditLog";
import { useAuditLog } from "../../../hooks/queries/admin";
import { useAuth } from "../../../hooks/useAuth";
import type { AuditAction } from "../../../types/api";
import { AdminPage } from "../shared/AdminPage";
import { AuditLogTable } from "../shared/AuditLogTable";
import styles from "./AdminAuditLogPage.module.css";

const PAGE_SIZE = 50;

export function AdminAuditLogPage() {
    const { permissions } = useAuth();
    const [action, setAction] = useState<AuditAction | undefined>(undefined);
    const [offset, setOffset] = useState(0);
    const log = useAuditLog({ action, limit: PAGE_SIZE, offset });
    const listing = log.data;

    function chooseAction(value: string) {
        setAction(isAuditAction(value) ? value : undefined);
        setOffset(0);
    }

    return (
        <AdminPage title="Audit log" description="A record of what staff changed on the site, and when.">
            <div className={styles.filter}>
                <FormField label="Action">
                    {control => (
                        <Select {...control} value={action ?? ""} onChange={e => chooseAction(e.target.value)}>
                            <option value="">All actions</option>
                            {SITE_AUDIT_ACTIONS.map(option => (
                                <option key={option} value={option}>
                                    {AUDIT_ACTION_LABELS[option]}
                                </option>
                            ))}
                        </Select>
                    )}
                </FormField>
            </div>

            {log.error !== null && <LoadFailure message={log.error.message} onRetry={log.refetch} />}

            {listing === undefined && log.error === null && <p className="loading">Loading the audit log...</p>}

            {listing !== undefined && listing.total === 0 && (
                <p className={styles.empty}>{action === undefined ? "Nothing recorded yet." : "No entries match."}</p>
            )}

            {listing !== undefined && listing.entries.length > 0 && (
                <AuditLogTable
                    caption="Audit log"
                    entries={listing.entries}
                    showSubject
                    linkSubjects={permissions.includes("view_users")}
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
        </AdminPage>
    );
}
