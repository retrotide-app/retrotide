import { useId } from "react";
import { BoardGrid } from "../../components/board/BoardGrid/BoardGrid";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { BOARD_NOUN } from "../../domain/board/organisationBoards";
import { boardPath, newBoardPath } from "../../domain/board/paths";
import { useTeamBoards } from "../../hooks/queries/boards";
import type { OrganisationResponse, TeamResponse } from "../../types/api";
import { countOf } from "../../utils/chart";
import styles from "./TeamPage.module.css";

interface TeamBoardsSectionProps {
    organisation: OrganisationResponse;
    team: TeamResponse;
    createsBoards: boolean;
}

export function TeamBoardsSection({ organisation, team, createsBoards }: TeamBoardsSectionProps) {
    const headingId = useId();
    const boards = useTeamBoards(team.id);

    const loaded = boards.data !== undefined;
    const listed = boards.data?.boards ?? [];
    const showsGrid = loaded && (createsBoards || listed.length > 0);

    return (
        <section aria-labelledby={headingId} className={styles.section}>
            <div className={styles.sectionHeader}>
                <div className={styles.sectionHeading}>
                    <h2 id={headingId} className={styles.sectionTitle}>
                        Boards
                    </h2>
                    {loaded && <span className={styles.count}>{countOf(listed.length, BOARD_NOUN)}</span>}
                </div>
            </div>

            {boards.error !== null && <LoadFailure message={boards.error.message} onRetry={boards.refetch} />}

            {!loaded && boards.error === null && <p className="loading">Loading boards...</p>}

            {loaded && listed.length === 0 && !createsBoards && <p>This team has no boards yet.</p>}

            {showsGrid && (
                <BoardGrid
                    boards={listed}
                    boardHref={boardId => boardPath(organisation.slug, boardId)}
                    addHref={createsBoards ? newBoardPath(organisation.slug, team.id) : undefined}
                    label={`Boards for ${team.name}`}
                />
            )}
        </section>
    );
}
