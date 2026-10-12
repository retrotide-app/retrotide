import { useEffect } from "react";
import { useParams } from "react-router";
import { AddColumnForm } from "../../components/board/AddColumnForm";
import colours from "../../components/board/boardColours.module.css";
import { ColumnTools } from "../../components/board/ColumnTools";
import { ParticipantList } from "../../components/board/ParticipantList";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { Panel } from "../../components/Panel/Panel";
import { boardActions, boardInteractions, boardManagementActions, isReadOnly } from "../../domain/board/capabilities";
import { boardPath } from "../../domain/board/paths";
import { compareByPosition } from "../../domain/board/sort";
import { isDeleted } from "../../domain/organisations";
import { useBoardView } from "../../hooks/board/useBoardView";
import { useJoinBoard } from "../../hooks/mutations/boards";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import type { BoardSnapshot, OrganisationResponse } from "../../types/api";
import { DeletedOrganisation } from "../org/shared/DeletedOrganisation";
import { OrgPage } from "../org/shared/OrgPage";
import { BoardBackgroundSection } from "./BoardBackgroundSection";
import { BoardDescription } from "./BoardDescription";
import { BoardInteractionSettings } from "./BoardInteractionSettings";
import { BoardManagementSection } from "./BoardManagementSection";
import styles from "./BoardSettingsPage.module.css";
import { CsvImportSection } from "./CsvImportSection";
import { SaveAsTemplateSection } from "./SaveAsTemplateSection";

const PAGE_TITLE = "Board settings";

interface BoardSettingsProps {
    organisation: OrganisationResponse;
    boardId: string;
}

interface ColumnSettingsProps {
    snapshot: BoardSnapshot;
}

function ColumnSettings({ snapshot }: ColumnSettingsProps) {
    const columns = snapshot.columns.toSorted(compareByPosition);

    return (
        <Panel title="Columns" headingLevel={2} description="The columns everyone writes cards in, left to right.">
            {columns.length === 0 && <p className={styles.muted}>This board has no columns yet.</p>}
            {columns.length > 0 && (
                <ol className={styles.columns} aria-label="Columns">
                    {columns.map(column => (
                        <li key={column.id} className={`${styles.column} ${colours[column.colour]}`}>
                            <div className={styles.columnText}>
                                <span className={styles.columnTitle} dir="auto">
                                    {column.title}
                                </span>
                                {column.description !== "" && (
                                    <span className={styles.muted} dir="auto">
                                        {column.description}
                                    </span>
                                )}
                            </div>
                            <ColumnTools snapshot={snapshot} column={column} />
                        </li>
                    ))}
                </ol>
            )}
            <AddColumnForm boardId={snapshot.board_id} />
        </Panel>
    );
}

function settingsRefusal({ board }: BoardSnapshot): string {
    if (board.closed) {
        return "This retro has ended, so its settings can no longer change.";
    }

    if (isReadOnly(board)) {
        return "This board is archived, so its settings can no longer change.";
    }

    return "Only the board's facilitators can change its settings.";
}

function BoardSettings({ organisation, boardId }: BoardSettingsProps) {
    const board = useBoardView(boardId);
    const join = useJoinBoard(boardId);
    const { mutate: joinBoard } = join;

    useEffect(() => {
        joinBoard();
    }, [joinBoard]);

    const snapshot = board.data;
    const back = <ButtonLink to={boardPath(organisation.slug, boardId)}>Back to board</ButtonLink>;

    if (snapshot === undefined) {
        return (
            <OrgPage title={PAGE_TITLE} status={back}>
                {board.error !== null && <LoadFailure message={board.error.message} onRetry={board.refetch} />}
                {board.error === null && <p className="loading">Loading board...</p>}
            </OrgPage>
        );
    }

    const actions = boardActions(snapshot);
    const interactions = boardInteractions(snapshot);
    const management = boardManagementActions(snapshot);
    const manages = management.archive || management.remove || management.move;
    const readOnly = isReadOnly(snapshot.board);

    if (readOnly || (!actions.manageColumns && !actions.describeBoard && !interactions.facilitate)) {
        return (
            <OrgPage title={PAGE_TITLE} status={back}>
                <div className={styles.sections}>
                    <p>{settingsRefusal(snapshot)}</p>
                    {manages && <BoardManagementSection organisation={organisation} snapshot={snapshot} />}
                </div>
            </OrgPage>
        );
    }

    return (
        <OrgPage
            title={PAGE_TITLE}
            description={
                <>
                    Set up <bdi>{snapshot.board.title}</bdi>: its description, columns, voting and cards. Everyone on
                    the board sees the changes.
                </>
            }
            status={back}
        >
            <div className={styles.sections}>
                {actions.describeBoard && (
                    <Panel title="Description" headingLevel={2} description="Shown at the top of the board.">
                        <BoardDescription board={snapshot.board} editable={join.isSuccess} />
                    </Panel>
                )}
                {actions.manageColumns && <ColumnSettings snapshot={snapshot} />}
                {management.importCards && <CsvImportSection snapshot={snapshot} />}
                {interactions.facilitate && <BoardInteractionSettings snapshot={snapshot} />}
                {management.background && <BoardBackgroundSection snapshot={snapshot} />}
                {interactions.facilitate && (
                    <Panel
                        title="People"
                        headingLevel={2}
                        description="Everyone who has joined the board. A co-facilitator runs the board with you."
                    >
                        <ParticipantList snapshot={snapshot} />
                    </Panel>
                )}
                {actions.describeBoard && <SaveAsTemplateSection organisation={organisation} board={snapshot.board} />}
                {manages && <BoardManagementSection organisation={organisation} snapshot={snapshot} />}
            </div>
        </OrgPage>
    );
}

export function BoardSettingsPage() {
    const { boardId = "" } = useParams();
    const { current } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return <BoardSettings key={`${current.id}/${boardId}`} organisation={current} boardId={boardId} />;
}
