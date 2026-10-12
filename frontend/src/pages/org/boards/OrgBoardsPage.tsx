import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import {
    boardCountLabels,
    boardCounts,
    type BoardDisplay,
    boardSections,
    boardTeamOptions,
    DEFAULT_BOARD_QUERY,
    isFiltering,
    toggledSelection,
} from "../../../domain/board/organisationBoards";
import { orgRoleGrants } from "../../../domain/organisationRoles";
import { isDeleted } from "../../../domain/organisations";
import { usePinBoard } from "../../../hooks/mutations/boardManagement";
import { useOrganisationBoards } from "../../../hooks/queries/boards";
import { useTeams } from "../../../hooks/queries/teams";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import type { OrganisationResponse, OrgRole } from "../../../types/api";
import { ActionFailure } from "../shared/ActionFailure";
import { DeletedOrganisation } from "../shared/DeletedOrganisation";
import { OrgPage } from "../shared/OrgPage";
import { BoardsSection } from "./BoardsSection";
import { BoardsToolbar } from "./BoardsToolbar";
import { BulkBoardBar } from "./BulkBoardBar";
import { DeletedBoardsSection } from "./DeletedBoardsSection";
import { FirstBoardGuide } from "./FirstBoardGuide";
import styles from "./OrgBoardsPage.module.css";

const PAGE_TITLE = "Boards";

const NO_SELECTION: ReadonlySet<string> = new Set();

interface BoardsHomeProps {
    organisation: OrganisationResponse;
    role: OrgRole | null;
}

function BoardsHome({ organisation, role }: BoardsHomeProps) {
    const boards = useOrganisationBoards(organisation.id);
    const teams = useTeams(organisation.id);
    const pin = usePinBoard(organisation.id);
    const [query, setQuery] = useState(DEFAULT_BOARD_QUERY);
    const [display, setDisplay] = useState<BoardDisplay>("grid");
    const [selecting, setSelecting] = useState(false);
    const [selected, setSelected] = useState<ReadonlySet<string>>(NO_SELECTION);

    const listed = boards.data?.boards ?? [];
    const teamList = teams.data?.teams ?? [];
    const loaded = boards.data !== undefined;
    const empty = loaded && listed.length === 0;
    const deletedView = query.status === "deleted";
    const member = role !== null;
    const sections = boardSections(listed, teamList, role, query);
    const nothingShown = loaded && !empty && !deletedView && sections.length === 0;
    const pinningId = pin.isPending ? (pin.variables?.boardId ?? null) : null;
    const choosing = selecting && !deletedView;
    const searchText = query.text.trim();

    const status = loaded && !empty && (
        <div className={styles.counts}>
            {boardCountLabels(boardCounts(listed)).map(label => (
                <StatusTag key={label} tone="neutral">
                    {label}
                </StatusTag>
            ))}
        </div>
    );

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="Every retro board you can open in this organisation. Open one to join in, or add a new board."
            status={status}
        >
            {boards.error !== null && <LoadFailure message={boards.error.message} onRetry={boards.refetch} />}
            {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}
            {pin.error !== null && <ActionFailure error={pin.error} />}

            {!loaded && boards.error === null && <p className="loading">Loading boards...</p>}

            {loaded && (!empty || deletedView) && (
                <BoardsToolbar
                    query={query}
                    onQueryChange={setQuery}
                    display={display}
                    onDisplayChange={setDisplay}
                    teamOptions={boardTeamOptions(teamList, role)}
                    selecting={choosing}
                    onSelectingChange={
                        member && !deletedView
                            ? next => {
                                  setSelecting(next);
                                  setSelected(NO_SELECTION);
                              }
                            : undefined
                    }
                />
            )}

            {choosing && (
                <BulkBoardBar
                    organisation={organisation}
                    boards={listed}
                    selected={selected}
                    onSelectionChange={setSelected}
                />
            )}

            {empty && !deletedView && (
                <>
                    <FirstBoardGuide organisation={organisation} canCreate={orgRoleGrants(role, "org.create_board")} />
                    {member && (
                        <div className={styles.emptyActions}>
                            <Button
                                type="button"
                                size="small"
                                variant="ghost"
                                onClick={() => setQuery({ ...query, status: "deleted" })}
                            >
                                Recently deleted boards
                            </Button>
                        </div>
                    )}
                </>
            )}

            {deletedView && <DeletedBoardsSection organisation={organisation} />}

            {loaded &&
                !empty &&
                !deletedView &&
                sections.map(section => (
                    <BoardsSection
                        key={section.key}
                        organisation={organisation}
                        section={section}
                        display={display}
                        onPin={
                            member
                                ? board => pin.mutate({ boardId: board.id, pinned: board.pinned !== true })
                                : undefined
                        }
                        pinningId={pinningId}
                        selected={choosing ? selected : undefined}
                        onSelect={
                            choosing ? board => setSelected(current => toggledSelection(current, board.id)) : undefined
                        }
                    />
                ))}

            {nothingShown && searchText !== "" && (
                <p className={styles.hint}>
                    No boards match &ldquo;<bdi>{searchText}</bdi>&rdquo;.
                </p>
            )}
            {nothingShown && searchText === "" && (
                <p className={styles.hint}>
                    {isFiltering(query) ? "No boards match these filters." : "No boards to show."}
                </p>
            )}
        </OrgPage>
    );
}

export function OrgBoardsPage() {
    const { current, role } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return <BoardsHome key={current.id} organisation={current} role={role} />;
}
