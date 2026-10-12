import type { ActionItemView, PersonView, PreviousActionView } from "./actionItem";
import type { SurveyView } from "./survey";

export type ReactionKey =
    | "thumbs_up"
    | "thumbs_down"
    | "heart"
    | "laugh"
    | "wow"
    | "sad"
    | "angry"
    | "party"
    | "rocket"
    | "fire"
    | "clap"
    | "thinking"
    | "eyes"
    | "hundred"
    | "star";

export interface ReactionCount {
    reaction: ReactionKey;
    count: number;
}

export interface CardReactionsView {
    card_id: string;
    reactions: ReactionCount[];
    my_reactions: ReactionKey[];
}

export type Colour = "green" | "red" | "blue" | "amber" | "purple" | "teal" | "pink" | "grey";

export type Anonymity = "named" | "anonymous";

export type BoardRole = "facilitator" | "participant";

export type BoardCapability = "board.view" | "board.participate" | "board.facilitate" | "board.manage" | "board.export";

export type BoardStage = "write" | "vote" | "discuss" | "actions" | "end";

export interface BoardCommandRequest {
    op_id: string;
}

export interface CreateBoardRequest {
    title: string;
    template_key: string;
    saved_template_id?: string;
    context?: string;
    anonymity?: Anonymity;
    cards_shown?: boolean;
    team_id?: string;
}

export interface DescribeBoardRequest extends BoardCommandRequest {
    context: string;
}

export interface ColumnFieldsRequest extends BoardCommandRequest {
    title: string;
    description?: string;
    colour?: Colour;
}

export interface CreateColumnRequest extends ColumnFieldsRequest {
    id: string;
}

export interface MoveColumnRequest extends BoardCommandRequest {
    after_column_id: string | null;
}

export interface CreateCardRequest extends BoardCommandRequest {
    id: string;
    column_id: string;
    body: string;
    colour?: Colour | null;
    mentions?: string[];
}

export interface EditCardRequest extends BoardCommandRequest {
    body: string;
    mentions: string[];
}

export interface RecolourCardRequest extends BoardCommandRequest {
    colour: Colour | null;
}

export interface MoveCardRequest extends BoardCommandRequest {
    column_id: string;
    after_card_id: string | null;
}

export interface MergeCardRequest extends BoardCommandRequest {
    target_card_id: string;
}

export interface CardPinRequest extends BoardCommandRequest {
    pinned: boolean;
}

export interface CreateCommentRequest extends BoardCommandRequest {
    id: string;
    body: string;
    mentions?: string[];
}

export interface EditCommentRequest extends BoardCommandRequest {
    body: string;
    mentions: string[];
}

export type SortMode = "manual" | "votes";

export interface BoardSettingsView {
    voting_open: boolean;
    vote_counts_hidden: boolean;
    max_votes: number;
    max_votes_per_card: number;
    comments_enabled: boolean;
    merging_enabled: boolean;
    sort_mode: SortMode;
    cards_locked: boolean;
    moves_locked: boolean;
    board_locked: boolean;
    timer_play_sound: boolean;
    timer_auto_lock: boolean;
    sort_enforced: boolean;
    anyone_can_edit: boolean;
    show_card_dates: boolean;
}

export type BoardSettingsRequest = BoardCommandRequest & Partial<BoardSettingsView>;

export interface CardSpotlightRequest extends BoardCommandRequest {
    card_id: string | null;
}

export interface ColumnFocusRequest extends BoardCommandRequest {
    column_id: string | null;
}

export interface ColumnVoteLimitRequest extends BoardCommandRequest {
    max_votes: number | null;
}

export type TimerAction = "start" | "pause" | "resume" | "add_minute" | "stop";

export interface TimerRequest extends BoardCommandRequest {
    action: TimerAction;
    duration_ms?: number;
}

export interface ParticipantRoleRequest extends BoardCommandRequest {
    role: BoardRole;
}

export interface StageRequest extends BoardCommandRequest {
    stage: BoardStage;
}

export interface TimerView {
    duration_ms: number;
    ends_at: string | null;
    paused_remaining_ms: number | null;
}

export interface MeView {
    votes_remaining: number;
    my_votes: Record<string, number>;
    version?: number;
}

export interface VotesView {
    total: number | null;
}

export interface AuthorView {
    participant_id: string;
    display_name: string;
}

export interface CommentView {
    id: string;
    card_id: string;
    body: string;
    author: AuthorView | null;
    is_mine?: true;
    mentions?: PersonView[];
    created_at?: string;
    updated_at?: string;
}

export interface ColumnView {
    id: string;
    title: string;
    description: string;
    colour: Colour;
    position: string;
    max_votes: number | null;
    cards_revealed: boolean;
    hidden: boolean;
}

export interface SpotlightView {
    presenting_card_id: string | null;
    highlighted_card_id: string | null;
    focused_column_id: string | null;
}

export interface CardView {
    id: string;
    column_id: string;
    parent_card_id: string | null;
    position: string;
    body: string;
    colour: Colour | null;
    author: AuthorView | null;
    is_mine?: true;
    mentions?: PersonView[];
    hidden: boolean;
    votes: VotesView;
    created_at?: string;
    updated_at?: string;
    pinned?: true;
    image_path?: string;
    reactions?: ReactionCount[];
    my_reactions?: ReactionKey[];
}

export interface BoardCommandResponse {
    version: number;
    replayed?: true;
    id?: string;
    card?: CardView;
    column?: ColumnView;
    comment?: CommentView;
    me?: MeView;
    action_item?: ActionItemView;
    previous_action?: PreviousActionView;
    reactions?: CardReactionsView;
    survey?: SurveyView;
}

export interface BoardView {
    id: string;
    team_id?: string;
    title: string;
    context: string;
    template_key: string;
    anonymity: Anonymity;
    cards_hidden: boolean;
    stage: BoardStage;
    closed: boolean;
    show_prime_directive: boolean;
    creator_name?: string;
    created_at?: string;
    archived_at?: string;
    background_path?: string;
}

export interface ViewerView {
    participant_id: string | null;
    role: BoardRole;
    capabilities: BoardCapability[];
}

export interface ParticipantView {
    participant_id: string;
    display_name: string;
    role: BoardRole;
}

export interface BoardSnapshot {
    board_id: string;
    version: number;
    board: BoardView;
    settings: BoardSettingsView;
    viewer: ViewerView;
    me: MeView;
    columns: ColumnView[];
    cards: CardView[];
    comments: CommentView[];
    hidden_counts: Record<string, number>;
    participants: ParticipantView[];
    timer: TimerView | null;
    action_items: ActionItemView[];
    previous_actions: PreviousActionView[];
    spotlight: SpotlightView;
    surveys: SurveyView[];
    server_now?: string;
}

export interface BoardListItem {
    id: string;
    team_id: string;
    title: string;
    creator_name?: string;
    created_at?: string;
    archived_at?: string;
    ended?: true;
}

export interface BoardListResponse {
    boards: BoardListItem[];
}

export interface OrgBoardListItem {
    id: string;
    team_id?: string;
    team_name?: string;
    title: string;
    template_key: string;
    creator_name?: string;
    created_at?: string;
    archived_at?: string;
    ended?: true;
    pinned?: true;
}

export interface OrgBoardListResponse {
    boards: OrgBoardListItem[];
}
