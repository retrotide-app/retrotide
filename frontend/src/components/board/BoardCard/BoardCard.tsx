import { Link } from "react-router";
import type { BoardListItem } from "../../../types/api";
import { formatDate } from "../../../utils/time";
import { Button } from "../../Button/Button";
import { StatusTag } from "../../StatusTag/StatusTag";
import styles from "./BoardCard.module.css";

export type BoardCardBoard = Pick<
    BoardListItem,
    "id" | "title" | "creator_name" | "created_at" | "archived_at" | "ended"
> & {
    pinned?: true;
};

interface BoardCardProps {
    board: BoardCardBoard;
    href: string;
    onPin?: (board: BoardCardBoard) => void;
    pinBusy?: boolean;
    selected?: boolean;
    onSelect?: (board: BoardCardBoard) => void;
}

const SELECTED_MARK = "✓";

const PIN_MARK = "📌";

function cardClassName(archived: boolean, selected: boolean): string {
    return [styles.card, archived ? styles.archived : "", selected ? styles.selected : ""].filter(Boolean).join(" ");
}

export function BoardCard({ board, href, onPin, pinBusy = false, selected = false, onSelect }: BoardCardProps) {
    const archived = board.archived_at !== undefined;
    const pinned = board.pinned === true;
    const pinLabel = pinned ? "Unpin board" : "Pin board";
    const controls = onPin !== undefined || onSelect !== undefined;

    return (
        <article className={cardClassName(archived, selected)}>
            <div className={styles.top}>
                <h3 className={styles.title} dir="auto">
                    <Link to={href} className={styles.link}>
                        <bdi>{board.title}</bdi>
                    </Link>
                </h3>
                {controls && (
                    <div className={styles.controls}>
                        {onSelect !== undefined && (
                            <Button
                                type="button"
                                size="small"
                                variant={selected ? "primary" : "secondary"}
                                role="checkbox"
                                aria-checked={selected}
                                aria-label={`Select ${board.title}`}
                                onClick={() => onSelect(board)}
                            >
                                <span aria-hidden="true">{selected ? SELECTED_MARK : ""}</span>
                            </Button>
                        )}
                        {onPin !== undefined && (
                            <Button
                                type="button"
                                size="small"
                                variant={pinned ? "primary" : "ghost"}
                                aria-pressed={pinned}
                                aria-label={pinLabel}
                                title={pinLabel}
                                disabled={pinBusy}
                                onClick={() => onPin(board)}
                            >
                                <span aria-hidden="true">{PIN_MARK}</span>
                            </Button>
                        )}
                    </div>
                )}
            </div>
            {board.creator_name !== undefined && (
                <p className={styles.creator}>
                    Created by <bdi>{board.creator_name}</bdi>
                </p>
            )}
            <div className={styles.details}>
                {board.created_at !== undefined && (
                    <time dateTime={board.created_at}>{formatDate(board.created_at)}</time>
                )}
                {board.ended === true && <StatusTag tone="neutral">Ended</StatusTag>}
                {archived && <StatusTag tone="neutral">Archived</StatusTag>}
            </div>
        </article>
    );
}
