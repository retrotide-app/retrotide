import { DataTable } from "../../../components/DataTable/DataTable";
import { TextLink } from "../../../components/TextLink/TextLink";
import { auditActionLabel, auditActorName, auditDetails } from "../../../domain/auditLog";
import type { AuditLogEntryResponse } from "../../../types/api";
import { formatExactDateTime } from "../../../utils/time";
import styles from "./AuditLogTable.module.css";

interface AuditLogTableProps {
    caption: string;
    entries: readonly AuditLogEntryResponse[];
    showSubject: boolean;
    linkSubjects: boolean;
}

interface SubjectCellProps {
    entry: AuditLogEntryResponse;
    link: boolean;
}

function SubjectCell({ entry, link }: SubjectCellProps) {
    if (entry.subject_id === undefined || entry.subject_id === "") {
        return null;
    }

    if (entry.subject_name === undefined || entry.subject_name === "") {
        return <span className={styles.muted}>Deleted account</span>;
    }

    if (!link) {
        return <bdi>{entry.subject_name}</bdi>;
    }

    return (
        <TextLink to={`/admin/users/${encodeURIComponent(entry.subject_id)}`}>
            <bdi>{entry.subject_name}</bdi>
        </TextLink>
    );
}

export function AuditLogTable({ caption, entries, showSubject, linkSubjects }: AuditLogTableProps) {
    return (
        <DataTable caption={caption}>
            <thead>
                <tr>
                    <th scope="col">When</th>
                    <th scope="col">Action</th>
                    {showSubject && <th scope="col">User</th>}
                    <th scope="col">By</th>
                    <th scope="col">Details</th>
                </tr>
            </thead>
            <tbody>
                {entries.map(entry => {
                    const details = auditDetails(entry);

                    return (
                        <tr key={entry.id}>
                            <td className={styles.when}>
                                <time dateTime={entry.created_at}>{formatExactDateTime(entry.created_at)}</time>
                            </td>
                            <td>{auditActionLabel(entry.action)}</td>
                            {showSubject && (
                                <td>
                                    <SubjectCell entry={entry} link={linkSubjects} />
                                </td>
                            )}
                            <td>
                                <bdi>{auditActorName(entry)}</bdi>
                            </td>
                            <td>
                                {details.length > 0 && (
                                    <dl className={styles.details}>
                                        {details.map(detail => (
                                            <div key={detail.key} className={styles.detail}>
                                                <dt className={styles.detailLabel}>{detail.label}</dt>
                                                <dd className={styles.detailValue} dir="auto">
                                                    {detail.value}
                                                </dd>
                                            </div>
                                        ))}
                                    </dl>
                                )}
                            </td>
                        </tr>
                    );
                })}
            </tbody>
        </DataTable>
    );
}
