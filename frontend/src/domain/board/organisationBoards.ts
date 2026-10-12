import type { BulkBoardResult, OrgBoardListItem, OrgRole, TeamResponse } from "../../types/api";
import { COUNT_FORMAT, countOf, type Plural } from "../../utils/chart";
import { orgRoleGrants, teamRoleGrants } from "../organisationRoles";
import { isArchived } from "../organisations";

export type BoardSort = "newest" | "oldest";

export type BoardDisplay = "grid" | "list";

export type BoardStatus = "active" | "ended" | "archived" | "all" | "deleted";

export interface BoardQuery {
    text: string;
    sort: BoardSort;
    team: string;
    status: BoardStatus;
    pinnedOnly: boolean;
}

export interface BoardSection {
    key: string;
    teamId: string | null;
    title: string;
    hint: string;
    boards: OrgBoardListItem[];
    canAdd: boolean;
    archived: boolean;
}

export interface BoardCounts {
    live: number;
    archived: number;
}

export interface BoardOption<T extends string> {
    value: T;
    label: string;
}

export interface BulkFailure {
    boardId: string;
    title: string;
    reason: string;
}

export const ORGANISATION_SECTION_KEY = "organisation";

export const ORGANISATION_SECTION_TITLE = "Organisation boards";

export const PINNED_SECTION_KEY = "pinned";

export const PINNED_SECTION_TITLE = "Pinned";

export const ALL_TEAMS = "all";

export const ORGANISATION_TEAM = "organisation";

export const DEFAULT_BOARD_QUERY: BoardQuery = {
    text: "",
    sort: "newest",
    team: ALL_TEAMS,
    status: "active",
    pinnedOnly: false,
};

export const BOARD_SORT_OPTIONS: readonly BoardOption<BoardSort>[] = [
    { value: "newest", label: "Newest first" },
    { value: "oldest", label: "Oldest first" },
];

export const BOARD_STATUS_OPTIONS: readonly BoardOption<BoardStatus>[] = [
    { value: "active", label: "Active" },
    { value: "ended", label: "Ended" },
    { value: "archived", label: "Archived" },
    { value: "all", label: "All boards" },
    { value: "deleted", label: "Recently deleted" },
];

export const BOARD_DISPLAY_OPTIONS: readonly BoardOption<BoardDisplay>[] = [
    { value: "grid", label: "Grid" },
    { value: "list", label: "List" },
];

export const BOARD_NOUN: Plural = { one: "board", other: "boards" };

export const MEMBER_NOUN: Plural = { one: "member", other: "members" };

const ORGANISATION_SECTION_HINT = "Everyone in the organisation can open these and add cards.";

const TEAM_SECTION_HINT = "Only people on this team can open these.";

const ARCHIVED_TEAM_SECTION_HINT = "This team is archived, so nobody can add boards to it.";

const PINNED_SECTION_HINT = "Boards you pinned. They stay in their own sections too.";

const ALL_TEAMS_LABEL = "All teams";

const ORGANISATION_TEAM_LABEL = "Organisation boards";

const ORGANISATION_BOARD_LABEL = "Organisation board (no team)";

const CURRENT_TEAM_LABEL = "Its current team";

export function boardSort(value: string): BoardSort | undefined {
    return BOARD_SORT_OPTIONS.find(option => option.value === value)?.value;
}

export function boardStatus(value: string): BoardStatus | undefined {
    return BOARD_STATUS_OPTIONS.find(option => option.value === value)?.value;
}

export function boardTeamOptions(teams: readonly TeamResponse[], orgRole: OrgRole | null): BoardOption<string>[] {
    const visible = teams
        .filter(team => teamRoleGrants(orgRole, team.role, "team.view"))
        .toSorted((left, right) => left.name.localeCompare(right.name) || left.id.localeCompare(right.id))
        .map(team => ({ value: team.id, label: team.name }));

    return [
        { value: ALL_TEAMS, label: ALL_TEAMS_LABEL },
        { value: ORGANISATION_TEAM, label: ORGANISATION_TEAM_LABEL },
        ...visible,
    ];
}

export function boardCounts(boards: readonly OrgBoardListItem[]): BoardCounts {
    const archived = boards.filter(board => board.archived_at !== undefined).length;

    return { live: boards.length - archived, archived };
}

export function boardCountLabels(counts: BoardCounts): string[] {
    const labels = [countOf(counts.live, BOARD_NOUN)];

    if (counts.archived > 0) {
        labels.push(`${COUNT_FORMAT.format(counts.archived)} archived`);
    }

    return labels;
}

export function splitEnded<T extends { ended?: true }>(boards: readonly T[]): { live: T[]; ended: T[] } {
    return {
        live: boards.filter(board => board.ended !== true),
        ended: boards.filter(board => board.ended === true),
    };
}

export function isFiltering(query: BoardQuery): boolean {
    return (
        query.text.trim() !== "" ||
        query.team !== DEFAULT_BOARD_QUERY.team ||
        query.status !== DEFAULT_BOARD_QUERY.status ||
        query.pinnedOnly
    );
}

function matchesStatus(board: OrgBoardListItem, status: BoardStatus): boolean {
    const archived = board.archived_at !== undefined;

    if (status === "all") {
        return true;
    }

    if (status === "deleted") {
        return false;
    }

    if (status === "archived") {
        return archived;
    }

    if (status === "ended") {
        return !archived && board.ended === true;
    }

    return !archived && board.ended !== true;
}

function matchesTeam(board: OrgBoardListItem, team: string): boolean {
    if (team === ALL_TEAMS) {
        return true;
    }

    if (team === ORGANISATION_TEAM) {
        return board.team_id === undefined;
    }

    return board.team_id === team;
}

export function shownBoards(boards: readonly OrgBoardListItem[], query: BoardQuery): OrgBoardListItem[] {
    const text = query.text.trim().toLocaleLowerCase();

    const kept = boards.filter(
        board =>
            matchesStatus(board, query.status) &&
            matchesTeam(board, query.team) &&
            (!query.pinnedOnly || board.pinned === true) &&
            board.title.toLocaleLowerCase().includes(text),
    );

    if (query.sort === "oldest") {
        return kept.reverse();
    }

    return kept;
}

function teamSection(team: TeamResponse, boards: OrgBoardListItem[], orgRole: OrgRole | null): BoardSection {
    const archived = isArchived(team);

    return {
        key: team.id,
        teamId: team.id,
        title: team.name,
        hint: archived ? ARCHIVED_TEAM_SECTION_HINT : TEAM_SECTION_HINT,
        boards,
        canAdd: !archived && teamRoleGrants(orgRole, team.role, "team.create_board"),
        archived,
    };
}

function groupByTeam(boards: readonly OrgBoardListItem[]): Map<string, OrgBoardListItem[]> {
    const byTeam = new Map<string, OrgBoardListItem[]>();

    for (const board of boards) {
        if (board.team_id === undefined) {
            continue;
        }

        const teamBoards = byTeam.get(board.team_id);

        if (teamBoards === undefined) {
            byTeam.set(board.team_id, [board]);
        } else {
            teamBoards.push(board);
        }
    }

    return byTeam;
}

function pinnedSection(boards: readonly OrgBoardListItem[]): BoardSection {
    return {
        key: PINNED_SECTION_KEY,
        teamId: null,
        title: PINNED_SECTION_TITLE,
        hint: PINNED_SECTION_HINT,
        boards: boards.filter(board => board.pinned === true),
        canAdd: false,
        archived: false,
    };
}

export function boardSections(
    boards: readonly OrgBoardListItem[],
    teams: readonly TeamResponse[],
    orgRole: OrgRole | null,
    query: BoardQuery,
): BoardSection[] {
    const shown = shownBoards(boards, query);
    const pinned = pinnedSection(shown);

    if (query.pinnedOnly) {
        return pinned.boards.length > 0 ? [pinned] : [];
    }

    const byTeam = groupByTeam(shown);
    const known = new Set(teams.map(team => team.id));

    const organisation: BoardSection = {
        key: ORGANISATION_SECTION_KEY,
        teamId: null,
        title: ORGANISATION_SECTION_TITLE,
        hint: ORGANISATION_SECTION_HINT,
        boards: shown.filter(board => board.team_id === undefined),
        canAdd: orgRoleGrants(orgRole, "org.create_board"),
        archived: false,
    };

    const listed = teams
        .filter(team => teamRoleGrants(orgRole, team.role, "team.view"))
        .map(team => teamSection(team, byTeam.get(team.id) ?? [], orgRole));

    const strays = [...byTeam.entries()]
        .filter(([teamId]) => !known.has(teamId))
        .map(([teamId, teamBoards]): BoardSection => ({
            key: teamId,
            teamId,
            title: teamBoards[0].team_name ?? "",
            hint: TEAM_SECTION_HINT,
            boards: teamBoards,
            canAdd: false,
            archived: false,
        }));

    const teamSections = [...listed, ...strays].sort(
        (left, right) => left.title.localeCompare(right.title) || left.key.localeCompare(right.key),
    );

    const filtering = isFiltering(query);

    const sections = [organisation, ...teamSections]
        .filter(section => section.boards.length > 0 || (!filtering && section.canAdd))
        .map(section => (filtering ? { ...section, canAdd: false } : section));

    return pinned.boards.length > 0 ? [pinned, ...sections] : sections;
}

export function boardHome(board: { team_id?: string }): string {
    return board.team_id === undefined || board.team_id === "" ? ORGANISATION_TEAM : board.team_id;
}

export function moveTargets(
    teams: readonly TeamResponse[],
    orgRole: OrgRole | null | undefined,
    current: string,
): BoardOption<string>[] {
    const creatable = teams
        .filter(team => !isArchived(team) && teamRoleGrants(orgRole, team.role, "team.create_board"))
        .toSorted((left, right) => left.name.localeCompare(right.name) || left.id.localeCompare(right.id))
        .map(team => ({ value: team.id, label: team.name }));

    const organisation = orgRoleGrants(orgRole, "org.create_board")
        ? [{ value: ORGANISATION_TEAM, label: ORGANISATION_BOARD_LABEL }]
        : [];

    const targets = [...organisation, ...creatable];

    if (targets.some(target => target.value === current)) {
        return targets;
    }

    const here =
        current === ORGANISATION_TEAM
            ? ORGANISATION_BOARD_LABEL
            : (teams.find(team => team.id === current)?.name ?? CURRENT_TEAM_LABEL);

    return [{ value: current, label: here }, ...targets];
}

export function toggledSelection(selected: ReadonlySet<string>, boardId: string): Set<string> {
    const next = new Set(selected);

    if (next.has(boardId)) {
        next.delete(boardId);
    } else {
        next.add(boardId);
    }

    return next;
}

export function bulkFailures(
    results: readonly BulkBoardResult[],
    boards: readonly Pick<OrgBoardListItem, "id" | "title">[],
): BulkFailure[] {
    const titles = new Map(boards.map(board => [board.id, board.title]));

    return results
        .filter(result => !result.ok)
        .map(result => ({
            boardId: result.board_id,
            title: titles.get(result.board_id) ?? result.board_id,
            reason: result.error ?? "",
        }));
}
