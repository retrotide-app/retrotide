import { Button } from "../../../components/Button/Button";
import { Input } from "../../../components/Input/Input";
import { Select } from "../../../components/Select/Select";
import { ToggleGroup } from "../../../components/ToggleGroup/ToggleGroup";
import { ToggleSwitch } from "../../../components/ToggleSwitch/ToggleSwitch";
import {
    BOARD_DISPLAY_OPTIONS,
    BOARD_SORT_OPTIONS,
    BOARD_STATUS_OPTIONS,
    type BoardDisplay,
    type BoardOption,
    type BoardQuery,
    boardSort,
    boardStatus,
} from "../../../domain/board/organisationBoards";
import styles from "./OrgBoardsPage.module.css";

interface BoardsToolbarProps {
    query: BoardQuery;
    onQueryChange: (query: BoardQuery) => void;
    display: BoardDisplay;
    onDisplayChange: (display: BoardDisplay) => void;
    teamOptions: readonly BoardOption<string>[];
    selecting?: boolean;
    onSelectingChange?: (selecting: boolean) => void;
}

export function BoardsToolbar({
    query,
    onQueryChange,
    display,
    onDisplayChange,
    teamOptions,
    selecting = false,
    onSelectingChange,
}: BoardsToolbarProps) {
    return (
        <div className={styles.toolbar}>
            <Input
                type="search"
                aria-label="Filter your boards"
                placeholder="Filter your boards"
                className={styles.filter}
                value={query.text}
                onChange={e => onQueryChange({ ...query, text: e.target.value })}
            />
            <Select
                size="small"
                aria-label="Team"
                className={styles.choice}
                value={query.team}
                onChange={e => onQueryChange({ ...query, team: e.target.value })}
            >
                {teamOptions.map(option => (
                    <option key={option.value} value={option.value}>
                        {option.label}
                    </option>
                ))}
            </Select>
            <Select
                size="small"
                aria-label="Status"
                className={styles.choice}
                value={query.status}
                onChange={e => {
                    const status = boardStatus(e.target.value);

                    if (status !== undefined) {
                        onQueryChange({ ...query, status });
                    }
                }}
            >
                {BOARD_STATUS_OPTIONS.map(option => (
                    <option key={option.value} value={option.value}>
                        {option.label}
                    </option>
                ))}
            </Select>
            <div className={styles.pinnedSwitch}>
                <ToggleSwitch
                    label="Pinned only"
                    enabled={query.pinnedOnly}
                    onChange={pinnedOnly => onQueryChange({ ...query, pinnedOnly })}
                />
            </div>
            <Select
                size="small"
                aria-label="Sort"
                className={styles.sort}
                value={query.sort}
                onChange={e => {
                    const sort = boardSort(e.target.value);

                    if (sort !== undefined) {
                        onQueryChange({ ...query, sort });
                    }
                }}
            >
                {BOARD_SORT_OPTIONS.map(option => (
                    <option key={option.value} value={option.value}>
                        {option.label}
                    </option>
                ))}
            </Select>
            {onSelectingChange !== undefined && (
                <Button
                    type="button"
                    size="small"
                    variant={selecting ? "primary" : "secondary"}
                    aria-pressed={selecting}
                    onClick={() => onSelectingChange(!selecting)}
                >
                    Select
                </Button>
            )}
            <ToggleGroup label="View" options={BOARD_DISPLAY_OPTIONS} value={display} onChange={onDisplayChange} />
        </div>
    );
}
