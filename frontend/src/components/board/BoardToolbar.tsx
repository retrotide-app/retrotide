import { useId } from "react";
import { boardInteractions } from "../../domain/board/capabilities";
import { authorChoices, cardSortFrom, shownSort, sortChoices } from "../../domain/board/cardFilter";
import { BOARD_LAYOUT_ID, BOARD_SEARCH_ID } from "../../domain/board/shortcuts";
import type { BoardLayout } from "../../hooks/board/useBoardLayout";
import type { CardFilterState } from "../../hooks/board/useCardFilter";
import type { BoardSnapshot } from "../../types/api";
import { Input } from "../Input/Input";
import { Select } from "../Select/Select";
import { StatusTag } from "../StatusTag/StatusTag";
import { BoardLayoutToggle } from "./BoardLayoutToggle";
import styles from "./BoardToolbar.module.css";

interface BoardToolbarProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cards: CardFilterState;
    layout: BoardLayout;
    onLayoutChange: (layout: BoardLayout) => void;
}

const EVERYONE = "";

export function BoardToolbar({ snapshot, writable, cards, layout, onLayoutChange }: BoardToolbarProps) {
    const { board, settings, me } = snapshot;
    const { filter, search, sortBy, showAuthor } = cards;
    const authors = authorChoices(snapshot);
    const voting = writable && boardInteractions(snapshot).vote;
    const sortEnforced = settings.sort_enforced;
    const sortNoteId = useId();

    return (
        <div className={styles.toolbar} role="toolbar" aria-label="Cards">
            <Input
                id={BOARD_SEARCH_ID}
                type="search"
                aria-label="Search cards"
                placeholder="Search cards"
                className={styles.search}
                value={filter.query}
                onChange={e => search(e.target.value)}
            />
            <Select
                size="small"
                aria-label="Sort cards"
                aria-describedby={sortEnforced ? sortNoteId : undefined}
                className={styles.select}
                disabled={sortEnforced}
                value={shownSort(filter, board)}
                onChange={e => {
                    const sort = cardSortFrom(e.target.value, board);

                    if (sort !== undefined) {
                        sortBy(sort);
                    }
                }}
            >
                {sortChoices(board).map(choice => (
                    <option key={choice.value} value={choice.value}>
                        {choice.label}
                    </option>
                ))}
            </Select>
            {sortEnforced && (
                <span id={sortNoteId}>
                    <StatusTag tone="neutral">Sort order set by the facilitator</StatusTag>
                </span>
            )}
            {authors.length > 0 && (
                <Select
                    size="small"
                    aria-label="Filter by author"
                    className={styles.select}
                    value={filter.authorId ?? EVERYONE}
                    onChange={e => showAuthor(e.target.value === EVERYONE ? null : e.target.value)}
                >
                    <option value={EVERYONE}>Everyone</option>
                    {authors.map(author => (
                        <option key={author.participant_id} value={author.participant_id}>
                            {author.display_name}
                        </option>
                    ))}
                </Select>
            )}
            {voting && (
                <span className={styles.votesLeft} role="status">
                    {`${me.votes_remaining} of ${settings.max_votes} votes left`}
                </span>
            )}
            <div id={BOARD_LAYOUT_ID} className={styles.layout}>
                <BoardLayoutToggle layout={layout} onChange={onLayoutChange} />
            </div>
        </div>
    );
}
