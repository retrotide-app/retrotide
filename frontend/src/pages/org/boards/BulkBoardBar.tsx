import { useMemo, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { BOARD_NOUN, type BulkFailure, bulkFailures } from "../../../domain/board/organisationBoards";
import { useBulkBoards } from "../../../hooks/mutations/boardManagement";
import { useBulkExportUrl } from "../../../hooks/queries/boardManagement";
import type { BulkBoardAction, OrganisationResponse, OrgBoardListItem } from "../../../types/api";
import { countOf } from "../../../utils/chart";
import { ActionFailure } from "../shared/ActionFailure";
import styles from "./OrgBoardsPage.module.css";

interface BulkBoardBarProps {
    organisation: OrganisationResponse;
    boards: readonly OrgBoardListItem[];
    selected: ReadonlySet<string>;
    onSelectionChange: (selected: ReadonlySet<string>) => void;
}

export function BulkBoardBar({ organisation, boards, selected, onSelectionChange }: BulkBoardBarProps) {
    const bulk = useBulkBoards(organisation.id);
    const [confirming, setConfirming] = useState(false);
    const [failures, setFailures] = useState<BulkFailure[]>([]);
    const chosen = useMemo(() => [...selected], [selected]);
    const exportUrl = useBulkExportUrl(organisation.id, chosen);
    const count = chosen.length;
    const idle = count === 0 || bulk.isPending;

    function run(action: BulkBoardAction) {
        setFailures([]);
        bulk.mutate(
            { action, boardIds: chosen },
            {
                onSuccess: response => {
                    const failed = bulkFailures(response.results, boards);

                    setFailures(failed);
                    setConfirming(false);
                    onSelectionChange(new Set(failed.map(failure => failure.boardId)));
                },
                onError: () => {
                    setConfirming(false);
                },
            },
        );
    }

    return (
        <div className={styles.toolbar} role="region" aria-label="Selected boards">
            <span className={styles.bulkCount}>{countOf(count, BOARD_NOUN)} selected</span>
            <Button type="button" size="small" disabled={idle} onClick={() => run("archive")}>
                Archive
            </Button>
            <Button type="button" size="small" disabled={idle} onClick={() => run("unarchive")}>
                Unarchive
            </Button>
            {count > 0 ? (
                <ButtonLink size="small" href={exportUrl} download>
                    Export
                </ButtonLink>
            ) : (
                <Button type="button" size="small" disabled>
                    Export
                </Button>
            )}
            <Button type="button" size="small" variant="danger" disabled={idle} onClick={() => setConfirming(true)}>
                Delete
            </Button>
            <Button
                type="button"
                size="small"
                variant="ghost"
                disabled={count === 0}
                onClick={() => onSelectionChange(new Set())}
            >
                Clear selection
            </Button>
            {bulk.error !== null && <ActionFailure error={bulk.error} />}
            {failures.length > 0 && (
                <div role="alert" className={styles.failures}>
                    <p>These boards could not be changed, so they stay selected:</p>
                    <ul className={styles.failureList}>
                        {failures.map(failure => (
                            <li key={failure.boardId}>
                                <bdi>{failure.title}</bdi>: {failure.reason}
                            </li>
                        ))}
                    </ul>
                </div>
            )}
            <ConfirmDialog
                open={confirming}
                title="Delete boards"
                body={
                    <p>
                        Delete {countOf(count, BOARD_NOUN)}? Everyone loses access straight away. You can restore them
                        from Recently deleted for 30 days.
                    </p>
                }
                confirmLabel="Delete boards"
                destructive
                busy={bulk.isPending}
                onConfirm={() => run("delete")}
                onCancel={() => setConfirming(false)}
            />
        </div>
    );
}
