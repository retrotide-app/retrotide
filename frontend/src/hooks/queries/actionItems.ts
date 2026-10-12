import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { listBoardPeople, listMentions, listOrganisationActionItems } from "../../api/endpoints/actionItems";
import { queryKeys } from "../../api/queryKeys";
import type {
    ActionItemListQuery,
    ActionItemListResponse,
    BoardPeopleResponse,
    MentionListResponse,
} from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useBoardPeople(boardId: string, enabled: boolean): QueryState<BoardPeopleResponse> {
    const people = useQuery({
        queryKey: queryKeys.boardPeople(boardId),
        queryFn: () => listBoardPeople(boardId),
        enabled: enabled && boardId !== "",
    });

    return queryState(people);
}

export function useOrganisationActionItems(
    orgId: string,
    query: ActionItemListQuery,
): QueryState<ActionItemListResponse> {
    const items = useQuery({
        queryKey: queryKeys.actionItems.list(orgId, query),
        queryFn: () => listOrganisationActionItems(orgId, query),
        enabled: orgId !== "",
        placeholderData: keepPreviousData,
    });

    return queryState(items);
}

export function useMentions(): QueryState<MentionListResponse> {
    const mentions = useQuery({
        queryKey: queryKeys.auth.mentions(),
        queryFn: listMentions,
    });

    return queryState(mentions);
}
