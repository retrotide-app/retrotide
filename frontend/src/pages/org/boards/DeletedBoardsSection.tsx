import { useId, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { DataTable } from "../../../components/DataTable/DataTable";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { Toast } from "../../../components/Toast/Toast";
import { BOARD_NOUN, ORGANISATION_SECTION_TITLE } from "../../../domain/board/organisationBoards";
import { useRestoreBoard } from "../../../hooks/mutations/boardManagement";
import { useDeletedBoards } from "../../../hooks/queries/boardManagement";
import type { OrganisationResponse } from "../../../types/api";
import { countOf } from "../../../utils/chart";
import { formatDate } from "../../../utils/time";
import { ActionFailure } from "../shared/ActionFailure";
import styles from "./OrgBoardsPage.module.css";

interface DeletedBoardsSectionProps {
    organisation: OrganisationResponse;
}

const SECTION_TITLE = "Recently deleted";

export function DeletedBoardsSection({ organisation }: DeletedBoardsSectionProps) {
    const deleted = useDeletedBoards(organisation.id, true);
    const restore = useRestoreBoard(organisation.id);
    const [restored, setRestored] = useState<string | null>(null);
    const titleId = useId();
    const hintId = useId();

    const boards = deleted.data?.boards ?? [];
    const loaded = deleted.data !== undefined;

    return (
        <section aria-labelledby={titleId} aria-describedby={hintId} className={styles.section}>
            <div className={styles.sectionHeader}>
                <h2 id={titleId} className={styles.sectionTitle}>
                    {SECTION_TITLE}
                </h2>
                {loaded && <StatusTag tone="neutral">{countOf(boards.length, BOARD_NOUN)}</StatusTag>}
            </div>
            <p id={hintId} className={styles.hint}>
                A deleted board waits here for 30 days. Restore it to bring it back as it was, with its cards, votes and
                comments.
            </p>

            {deleted.error !== null && <LoadFailure message={deleted.error.message} onRetry={deleted.refetch} />}
            {!loaded && deleted.error === null && <p className="loading">Loading deleted boards...</p>}
            {restore.error !== null && <ActionFailure error={restore.error} />}
            {restored !== null && (
                <Toast variant="success" onDismiss={() => setRestored(null)}>
                    Restored <bdi>{restored}</bdi>.
                </Toast>
            )}

            {loaded && boards.length === 0 && <p className={styles.hint}>No deleted boards to restore.</p>}

            {boards.length > 0 && (
                <DataTable caption="Recently deleted boards">
                    <thead>
                        <tr>
                            <th scope="col">Board</th>
                            <th scope="col">Team</th>
                            <th scope="col">Deleted</th>
                            <th scope="col">Restore until</th>
                            <th scope="col">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {boards.map(board => (
                            <tr key={board.id}>
                                <td>
                                    <bdi>{board.title}</bdi>
                                </td>
                                <td>
                                    <bdi>
                                        {board.team_id === undefined ? ORGANISATION_SECTION_TITLE : board.team_name}
                                    </bdi>
                                </td>
                                <td>
                                    <time dateTime={board.deleted_at}>{formatDate(board.deleted_at)}</time>
                                </td>
                                <td>
                                    <time dateTime={board.restore_until}>{formatDate(board.restore_until)}</time>
                                </td>
                                <td>
                                    <Button
                                        type="button"
                                        size="small"
                                        aria-label={`Restore ${board.title}`}
                                        disabled={restore.isPending}
                                        onClick={() => {
                                            setRestored(null);
                                            restore.mutate(board.id, { onSuccess: () => setRestored(board.title) });
                                        }}
                                    >
                                        Restore
                                    </Button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}
        </section>
    );
}
