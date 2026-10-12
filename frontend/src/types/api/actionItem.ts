import type { BoardCommandRequest } from "./board";

export type ActionItemStatus = "open" | "done" | "dropped";

export type ReviewOutcome = "done" | "carried" | "dropped";

export interface PersonView {
    user_id: string;
    display_name: string;
}

export interface ActionItemView {
    id: string;
    title: string;
    assignee: PersonView | null;
    mentions: PersonView[];
    due_on: string | null;
    status: ActionItemStatus;
    source_card_id: string | null;
    is_mine?: true;
}

export interface PreviousActionView extends ActionItemView {
    review: ReviewOutcome | null;
}

export interface ActionItemFieldsRequest extends BoardCommandRequest {
    title: string;
    assignee_user_id: string | null;
    due_on: string | null;
    mentions: string[];
}

export interface CreateActionItemRequest extends ActionItemFieldsRequest {
    id: string;
    source_card_id: string;
}

export interface ActionItemStatusRequest extends BoardCommandRequest {
    status: ActionItemStatus;
}

export interface ReviewActionItemRequest extends BoardCommandRequest {
    outcome: ReviewOutcome;
}

export interface BoardPeopleResponse {
    people: PersonView[];
}

export interface ActionItemBoardView {
    id: string;
    title: string;
}

export interface ActionItemListItem extends ActionItemView {
    board: ActionItemBoardView | null;
    team_id?: string;
    team_name?: string;
}

export interface ActionItemCountsView {
    open: number;
    done: number;
    dropped: number;
}

export interface ActionItemListResponse {
    action_items: ActionItemListItem[];
    total: number;
    limit: number;
    offset: number;
    counts: ActionItemCountsView;
    completion_rate: number | null;
}

export interface ActionItemListQuery {
    status?: ActionItemStatus;
    assignee?: string;
    team?: string;
    limit?: number;
    offset?: number;
}

export type MentionKind = "action_item" | "card" | "comment";

interface MentionPlaceView {
    org_id: string;
    org_slug: string;
    org_name: string;
    team_id?: string;
}

export interface ActionItemMentionView extends MentionPlaceView {
    kind: "action_item";
    action_item_id: string;
    title: string;
    status: ActionItemStatus;
    due_on: string | null;
    board_id?: string;
    board_title?: string;
}

export interface CardMentionView extends MentionPlaceView {
    kind: "card";
    card_id: string;
    body: string;
    board_id: string;
    board_title: string;
}

export interface CommentMentionView extends MentionPlaceView {
    kind: "comment";
    card_id: string;
    comment_id: string;
    body: string;
    board_id: string;
    board_title: string;
}

export type MentionView = ActionItemMentionView | CardMentionView | CommentMentionView;

export interface MentionListResponse {
    mentions: MentionView[];
}
