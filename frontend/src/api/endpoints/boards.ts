import type {
    BoardCommandRequest,
    BoardCommandResponse,
    BoardListResponse,
    BoardSettingsRequest,
    BoardSnapshot,
    CardSpotlightRequest,
    ColumnFieldsRequest,
    ColumnFocusRequest,
    ColumnVoteLimitRequest,
    CreateBoardRequest,
    CreateCardRequest,
    CreateColumnRequest,
    CreateCommentRequest,
    DescribeBoardRequest,
    EditCardRequest,
    EditCommentRequest,
    MergeCardRequest,
    MoveCardRequest,
    MoveColumnRequest,
    OrgBoardListResponse,
    ParticipantRoleRequest,
    RecolourCardRequest,
    StageRequest,
    TimerRequest,
} from "../../types/api";
import { apiDeleteWithBody, apiFetch, apiPost, apiPut, buildQueryString } from "../client";

function teamBoardsPath(teamId: string): string {
    return `/teams/${encodeURIComponent(teamId)}/boards`;
}

function organisationBoardsPath(orgId: string): string {
    return `/orgs/${encodeURIComponent(orgId)}/boards`;
}

function boardPath(boardId: string, action = ""): string {
    return `/boards/${encodeURIComponent(boardId)}${action}`;
}

function columnPath(boardId: string, columnId: string, action = ""): string {
    return boardPath(boardId, `/columns/${encodeURIComponent(columnId)}${action}`);
}

function cardPath(boardId: string, cardId: string, action = ""): string {
    return boardPath(boardId, `/cards/${encodeURIComponent(cardId)}${action}`);
}

function commentPath(boardId: string, commentId: string): string {
    return boardPath(boardId, `/comments/${encodeURIComponent(commentId)}`);
}

function participantPath(boardId: string, participantId: string, action = ""): string {
    return boardPath(boardId, `/participants/${encodeURIComponent(participantId)}${action}`);
}

export async function listTeamBoards(teamId: string): Promise<BoardListResponse> {
    return apiFetch<BoardListResponse>(teamBoardsPath(teamId));
}

export async function listOrganisationBoards(orgId: string): Promise<OrgBoardListResponse> {
    return apiFetch<OrgBoardListResponse>(organisationBoardsPath(orgId));
}

export async function createBoard(orgId: string, body: CreateBoardRequest): Promise<BoardSnapshot> {
    return apiPost<BoardSnapshot, CreateBoardRequest>(organisationBoardsPath(orgId), body);
}

export type BoardFetchReason = "reconnect";

export async function getBoard(
    boardId: string,
    knownVersion?: number,
    reason?: BoardFetchReason,
): Promise<BoardSnapshot | undefined> {
    return apiFetch<BoardSnapshot | undefined>(
        boardPath(boardId, buildQueryString({ known_version: knownVersion, reason })),
    );
}

export async function joinBoard(boardId: string): Promise<void> {
    return apiPost<void>(boardPath(boardId, "/participants/me"));
}

export async function describeBoard(boardId: string, body: DescribeBoardRequest): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, DescribeBoardRequest>(boardPath(boardId, "/context"), body);
}

export async function createColumn(boardId: string, body: CreateColumnRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CreateColumnRequest>(boardPath(boardId, "/columns"), body);
}

export async function updateColumn(
    boardId: string,
    columnId: string,
    body: ColumnFieldsRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, ColumnFieldsRequest>(columnPath(boardId, columnId), body);
}

export async function moveColumn(
    boardId: string,
    columnId: string,
    body: MoveColumnRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, MoveColumnRequest>(columnPath(boardId, columnId, "/move"), body);
}

export async function clearColumn(
    boardId: string,
    columnId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(columnPath(boardId, columnId, "/clear"), body);
}

export async function deleteColumn(
    boardId: string,
    columnId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(columnPath(boardId, columnId), body);
}

export async function createCard(boardId: string, body: CreateCardRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CreateCardRequest>(boardPath(boardId, "/cards"), body);
}

export async function editCard(boardId: string, cardId: string, body: EditCardRequest): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, EditCardRequest>(cardPath(boardId, cardId), body);
}

export async function recolourCard(
    boardId: string,
    cardId: string,
    body: RecolourCardRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, RecolourCardRequest>(cardPath(boardId, cardId, "/colour"), body);
}

export async function moveCard(boardId: string, cardId: string, body: MoveCardRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, MoveCardRequest>(cardPath(boardId, cardId, "/move"), body);
}

export async function deleteCard(
    boardId: string,
    cardId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(cardPath(boardId, cardId), body);
}

export async function castVote(
    boardId: string,
    cardId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(cardPath(boardId, cardId, "/votes"), body);
}

export async function removeVote(
    boardId: string,
    cardId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(cardPath(boardId, cardId, "/votes"), body);
}

export async function resetVotes(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId, "/votes/reset"), body);
}

export async function updateBoardSettings(boardId: string, body: BoardSettingsRequest): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, BoardSettingsRequest>(boardPath(boardId, "/settings"), body);
}

export async function mergeCard(
    boardId: string,
    cardId: string,
    body: MergeCardRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, MergeCardRequest>(cardPath(boardId, cardId, "/merge"), body);
}

export async function unmergeCard(
    boardId: string,
    cardId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(cardPath(boardId, cardId, "/unmerge"), body);
}

export async function createComment(
    boardId: string,
    cardId: string,
    body: CreateCommentRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CreateCommentRequest>(cardPath(boardId, cardId, "/comments"), body);
}

export async function editComment(
    boardId: string,
    commentId: string,
    body: EditCommentRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, EditCommentRequest>(commentPath(boardId, commentId), body);
}

export async function revealCards(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId, "/reveal"), body);
}

export async function hideCards(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId, "/hide"), body);
}

export async function changeStage(boardId: string, body: StageRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, StageRequest>(boardPath(boardId, "/stage"), body);
}

export async function runTimer(boardId: string, body: TimerRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, TimerRequest>(boardPath(boardId, "/timer"), body);
}

export async function closeBoard(boardId: string, body: BoardCommandRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(boardPath(boardId, "/close"), body);
}

export async function setParticipantRole(
    boardId: string,
    participantId: string,
    body: ParticipantRoleRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, ParticipantRoleRequest>(participantPath(boardId, participantId, "/role"), body);
}

export async function removeParticipant(
    boardId: string,
    participantId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(participantPath(boardId, participantId), body);
}

export async function deleteComment(
    boardId: string,
    commentId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiDeleteWithBody<BoardCommandResponse, BoardCommandRequest>(commentPath(boardId, commentId), body);
}

export async function revealColumn(
    boardId: string,
    columnId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(columnPath(boardId, columnId, "/reveal"), body);
}

export async function hideColumn(
    boardId: string,
    columnId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(columnPath(boardId, columnId, "/hide"), body);
}

export async function showColumn(
    boardId: string,
    columnId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(columnPath(boardId, columnId, "/show"), body);
}

export async function resetColumnVotes(
    boardId: string,
    columnId: string,
    body: BoardCommandRequest,
): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, BoardCommandRequest>(columnPath(boardId, columnId, "/votes/reset"), body);
}

export async function setColumnVoteLimit(
    boardId: string,
    columnId: string,
    body: ColumnVoteLimitRequest,
): Promise<BoardCommandResponse> {
    return apiPut<BoardCommandResponse, ColumnVoteLimitRequest>(columnPath(boardId, columnId, "/vote-limit"), body);
}

export async function presentCard(boardId: string, body: CardSpotlightRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CardSpotlightRequest>(boardPath(boardId, "/presentation"), body);
}

export async function highlightCard(boardId: string, body: CardSpotlightRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, CardSpotlightRequest>(boardPath(boardId, "/highlight"), body);
}

export async function focusColumn(boardId: string, body: ColumnFocusRequest): Promise<BoardCommandResponse> {
    return apiPost<BoardCommandResponse, ColumnFocusRequest>(boardPath(boardId, "/focus"), body);
}
