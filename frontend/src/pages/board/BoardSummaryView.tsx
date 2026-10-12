import { BoardExportMenu } from "../../components/board/BoardExportMenu";
import { BoardSummaryReport } from "../../components/board/summary/BoardSummaryReport";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../components/StatusTag/StatusTag";
import { TextLink } from "../../components/TextLink/TextLink";
import { useBoardSummary } from "../../hooks/board/useBoardSummary";
import { OrgPage } from "../org/shared/OrgPage";
import styles from "./BoardPage.module.css";

interface BoardSummaryViewProps {
    boardId: string;
    boardLink: string;
}

const PAGE_TITLE = "Board summary";

export function BoardSummaryView({ boardId, boardLink }: BoardSummaryViewProps) {
    const { data: summary, error, refetch } = useBoardSummary(boardId);

    if (summary === undefined) {
        return (
            <OrgPage title={PAGE_TITLE}>
                {error !== null && <LoadFailure message={error.message} onRetry={refetch} />}
                {error === null && <p className="loading">Loading summary...</p>}
            </OrgPage>
        );
    }

    return (
        <OrgPage
            title={summary.title}
            description="A read-only summary of the board: its cards by votes, their comments and the actions agreed."
            badge={<StatusTag tone="neutral">{summary.stage}</StatusTag>}
            status={summary.exportable ? <BoardExportMenu boardId={boardId} /> : undefined}
        >
            <p className={styles.muted}>
                <TextLink to={boardLink}>Back to board</TextLink>
            </p>
            <div className={styles.stack} data-layout="full-width">
                <BoardSummaryReport summary={summary} />
            </div>
        </OrgPage>
    );
}
