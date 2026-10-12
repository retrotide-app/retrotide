import type { BoardDisplay } from "../../../domain/board/organisationBoards";
import { AddBoardTile } from "../AddBoardTile/AddBoardTile";
import { BoardCard, type BoardCardBoard } from "../BoardCard/BoardCard";
import styles from "./BoardGrid.module.css";

interface BoardGridProps {
    boards: readonly BoardCardBoard[];
    boardHref: (boardId: string) => string;
    addHref?: string;
    display?: BoardDisplay;
    label?: string;
    labelledBy?: string;
    onPin?: (board: BoardCardBoard) => void;
    pinningId?: string | null;
    selected?: ReadonlySet<string>;
    onSelect?: (board: BoardCardBoard) => void;
}

export function BoardGrid({
    boards,
    boardHref,
    addHref,
    display = "grid",
    label,
    labelledBy,
    onPin,
    pinningId = null,
    selected,
    onSelect,
}: BoardGridProps) {
    return (
        <ul aria-label={label} aria-labelledby={labelledBy} className={display === "grid" ? styles.grid : styles.list}>
            {addHref !== undefined && (
                <li>
                    <AddBoardTile href={addHref} label="Add board" />
                </li>
            )}
            {boards.map(board => (
                <li key={board.id}>
                    <BoardCard
                        board={board}
                        href={boardHref(board.id)}
                        onPin={onPin}
                        pinBusy={pinningId === board.id}
                        selected={selected?.has(board.id) ?? false}
                        onSelect={onSelect}
                    />
                </li>
            ))}
        </ul>
    );
}
