import { useId } from "react";
import type { BoardCardBoard } from "../../../components/board/BoardCard/BoardCard";
import { BoardGrid } from "../../../components/board/BoardGrid/BoardGrid";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { BOARD_NOUN, type BoardDisplay, type BoardSection, splitEnded } from "../../../domain/board/organisationBoards";
import { boardPath, NEW_BOARD_NO_TEAM, newBoardPath } from "../../../domain/board/paths";
import type { OrganisationResponse } from "../../../types/api";
import { countOf } from "../../../utils/chart";
import styles from "./OrgBoardsPage.module.css";

interface BoardsSectionProps {
    organisation: OrganisationResponse;
    section: BoardSection;
    display: BoardDisplay;
    onPin?: (board: BoardCardBoard) => void;
    pinningId?: string | null;
    selected?: ReadonlySet<string>;
    onSelect?: (board: BoardCardBoard) => void;
}

export function BoardsSection({
    organisation,
    section,
    display,
    onPin,
    pinningId,
    selected,
    onSelect,
}: BoardsSectionProps) {
    const titleId = useId();
    const hintId = useId();
    const endedId = useId();
    const addPath = newBoardPath(organisation.slug, section.teamId ?? NEW_BOARD_NO_TEAM);
    const { live, ended } = splitEnded(section.boards);
    const href = (boardId: string) => boardPath(organisation.slug, boardId);
    const tiles = { onPin, pinningId, selected, onSelect };

    return (
        <section aria-labelledby={titleId} aria-describedby={hintId} className={styles.section}>
            <div className={styles.sectionHeader}>
                <h2 id={titleId} className={styles.sectionTitle} dir="auto">
                    <bdi>{section.title}</bdi>
                </h2>
                <StatusTag tone="neutral">{countOf(section.boards.length, BOARD_NOUN)}</StatusTag>
                {section.archived && <StatusTag tone="warning">Archived team</StatusTag>}
            </div>
            <p id={hintId} className={styles.hint}>
                {section.hint}
            </p>
            {(live.length > 0 || section.canAdd) && (
                <BoardGrid
                    boards={live}
                    boardHref={href}
                    addHref={section.canAdd ? addPath : undefined}
                    display={display}
                    labelledBy={titleId}
                    {...tiles}
                />
            )}
            {ended.length > 0 && (
                <>
                    <h3 id={endedId} className={styles.endedTitle}>
                        Ended retros
                    </h3>
                    <BoardGrid boards={ended} boardHref={href} display={display} labelledBy={endedId} {...tiles} />
                </>
            )}
        </section>
    );
}
