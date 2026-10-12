import { useQuery, useQueryClient } from "@tanstack/react-query";
import { boardSnapshotQuery } from "../../api/cache/boardSnapshot";
import { getBoardTemplate, listBoardTemplates } from "../../api/endpoints/boardTemplates";
import { listOrganisationBoards, listTeamBoards } from "../../api/endpoints/boards";
import { queryKeys } from "../../api/queryKeys";
import type {
    BoardListResponse,
    BoardSnapshot,
    OrgBoardListResponse,
    TemplateListResponse,
    TemplateResponse,
} from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useTeamBoards(teamId: string): QueryState<BoardListResponse> {
    const boards = useQuery({
        queryKey: queryKeys.boards.team(teamId),
        queryFn: () => listTeamBoards(teamId),
        enabled: teamId !== "",
    });

    return queryState(boards);
}

export function useOrganisationBoards(orgId: string): QueryState<OrgBoardListResponse> {
    const boards = useQuery({
        queryKey: queryKeys.boards.organisation(orgId),
        queryFn: () => listOrganisationBoards(orgId),
        enabled: orgId !== "",
    });

    return queryState(boards);
}

export function useBoard(boardId: string): QueryState<BoardSnapshot> {
    const qc = useQueryClient();
    const board = useQuery({ ...boardSnapshotQuery(qc, boardId), enabled: boardId !== "" });

    return queryState(board);
}

export function useBoardTemplates(): QueryState<TemplateListResponse> {
    const templates = useQuery({
        queryKey: queryKeys.boardTemplates(),
        queryFn: listBoardTemplates,
    });

    return queryState(templates);
}

export function useBoardTemplate(key: string): QueryState<TemplateResponse> {
    const template = useQuery({
        queryKey: queryKeys.boardTemplate(key),
        queryFn: () => getBoardTemplate(key),
        enabled: key !== "",
    });

    return queryState(template);
}
