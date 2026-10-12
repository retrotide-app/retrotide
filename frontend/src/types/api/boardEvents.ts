import type { ActionItemView, PreviousActionView } from "./actionItem";
import type {
    BoardRole,
    BoardSettingsView,
    BoardStage,
    CardView,
    ColumnView,
    CommentView,
    ParticipantView,
    ReactionCount,
    ReactionKey,
    SpotlightView,
    TimerView,
} from "./board";
import type { SurveyView } from "./survey";

export interface BoardRef {
    board_id: string;
}

export type BoardRefusalReason =
    | "not_participant"
    | "password_required"
    | "account_required"
    | "link_disabled"
    | "removed"
    | "archived";

export interface BoardJoinedPayload extends BoardRef {
    version: number;
    participant_id: string | null;
    role: BoardRole;
}

export interface BoardRefusalPayload extends BoardRef {
    reason: BoardRefusalReason;
}

export type ViewerState = "active" | "idle";

export interface PresenceViewer {
    participant_id: string;
    display_name: string;
    state?: ViewerState;
}

export interface BoardPresencePayload extends BoardRef {
    viewers: PresenceViewer[];
}

export interface PositionView {
    id: string;
    position: string;
}

export interface BoardEventHead<T extends string> {
    type: T;
    version: number;
}

export type ResyncEvent = BoardEventHead<"resync">;

export interface BoardUpdatedEvent extends BoardEventHead<"board_updated"> {
    title: string;
    context: string;
}

export interface ParticipantUpsertedEvent extends BoardEventHead<"participant_upserted"> {
    participant: ParticipantView;
}

export interface ColumnUpsertedEvent extends BoardEventHead<"column_upserted"> {
    column: ColumnView;
}

export interface ColumnMovedEvent extends BoardEventHead<"column_moved"> {
    column_id: string;
    position: string;
    rewritten: PositionView[];
}

export interface ColumnDeletedEvent extends BoardEventHead<"column_deleted"> {
    column_id: string;
}

export interface ColumnClearedEvent extends BoardEventHead<"column_cleared"> {
    column_id: string;
}

export interface CardUpsertedEvent extends BoardEventHead<"card_upserted"> {
    card: CardView;
}

export interface CardMovedEvent extends BoardEventHead<"card_moved"> {
    card_id: string;
    column_id: string;
    position: string;
    rewritten: PositionView[];
}

export interface CardDeletedEvent extends BoardEventHead<"card_deleted"> {
    card_id: string;
}

export interface CardsMergedEvent extends BoardEventHead<"cards_merged"> {
    card_id: string;
    parent_card_id: string;
    column_id: string;
    position: string;
    restacked: string[];
}

export interface CardUnmergedEvent extends BoardEventHead<"card_unmerged"> {
    card_id: string;
    column_id: string;
    position: string;
    rewritten: PositionView[];
}

export interface HiddenCountsEvent extends BoardEventHead<"hidden_counts"> {
    hidden_counts: Record<string, number>;
}

export interface SettingsChangedEvent extends BoardEventHead<"settings_changed"> {
    settings: BoardSettingsView;
}

export interface VotesChangedEvent extends BoardEventHead<"votes_changed"> {
    card_id: string;
    total: number | null;
}

export type VotesResetEvent = BoardEventHead<"votes_reset">;

export interface CommentUpsertedEvent extends BoardEventHead<"comment_upserted"> {
    comment: CommentView;
}

export interface CommentDeletedEvent extends BoardEventHead<"comment_deleted"> {
    comment_id: string;
    card_id: string;
}

export interface ParticipantRemovedEvent extends BoardEventHead<"participant_removed"> {
    participant_id: string;
}

export interface CardsRevealedEvent extends BoardEventHead<"cards_revealed"> {
    stage: BoardStage;
    cards: CardView[];
    comments: CommentView[];
}

export interface StageChangedEvent extends BoardEventHead<"stage_changed"> {
    stage: BoardStage;
}

export interface CardsHiddenEvent extends BoardEventHead<"cards_hidden"> {
    hidden_counts: Record<string, number>;
}

export interface TimerChangedEvent extends BoardEventHead<"timer_changed"> {
    timer: TimerView | null;
    server_now: string;
}

export interface ActionItemUpsertedEvent extends BoardEventHead<"action_item_upserted"> {
    action_item: ActionItemView;
}

export interface ActionItemRemovedEvent extends BoardEventHead<"action_item_removed"> {
    action_item_id: string;
}

export interface PreviousActionUpsertedEvent extends BoardEventHead<"previous_action_upserted"> {
    previous_action: PreviousActionView;
}

export interface ColumnRevealedEvent extends BoardEventHead<"column_revealed"> {
    column_id: string;
    cards: CardView[];
    comments: CommentView[];
}

export interface ColumnHiddenEvent extends BoardEventHead<"column_hidden"> {
    column_id: string;
}

export interface ColumnVotesResetEvent extends BoardEventHead<"column_votes_reset"> {
    column_id: string;
}

export interface SpotlightChangedEvent extends BoardEventHead<"spotlight_changed"> {
    spotlight: SpotlightView;
}

export interface CardReactionsChangedEvent extends BoardEventHead<"card_reactions_changed"> {
    card_id: string;
    reactions: ReactionCount[];
    my_reactions: ReactionKey[];
}

export interface CardPinnedEvent extends BoardEventHead<"card_pinned"> {
    card_id: string;
    pinned: boolean;
}

export interface CardImageChangedEvent extends BoardEventHead<"card_image_changed"> {
    card_id: string;
    image_path: string | null;
}

export interface BoardBackgroundChangedEvent extends BoardEventHead<"board_background_changed"> {
    background_path: string | null;
}

export interface SurveyUpsertedEvent extends BoardEventHead<"survey_upserted"> {
    survey: SurveyView;
}

export interface SurveyRemovedEvent extends BoardEventHead<"survey_removed"> {
    survey_id: string;
}

export type BoardEvent =
    | ResyncEvent
    | BoardUpdatedEvent
    | SettingsChangedEvent
    | ParticipantUpsertedEvent
    | ColumnUpsertedEvent
    | ColumnMovedEvent
    | ColumnDeletedEvent
    | ColumnClearedEvent
    | CardUpsertedEvent
    | CardMovedEvent
    | CardDeletedEvent
    | CardsMergedEvent
    | CardUnmergedEvent
    | HiddenCountsEvent
    | VotesChangedEvent
    | VotesResetEvent
    | CommentUpsertedEvent
    | CommentDeletedEvent
    | ParticipantRemovedEvent
    | CardsRevealedEvent
    | CardsHiddenEvent
    | StageChangedEvent
    | TimerChangedEvent
    | ActionItemUpsertedEvent
    | ActionItemRemovedEvent
    | PreviousActionUpsertedEvent
    | ColumnRevealedEvent
    | ColumnHiddenEvent
    | ColumnVotesResetEvent
    | SpotlightChangedEvent
    | CardReactionsChangedEvent
    | CardPinnedEvent
    | CardImageChangedEvent
    | BoardBackgroundChangedEvent
    | SurveyUpsertedEvent
    | SurveyRemovedEvent;

export type BoardEventType = BoardEvent["type"];

export interface BoardEventsFrame {
    board_id: string;
    from: number;
    to: number;
    events: BoardEvent[];
}

export interface TeamActionItemsChangedPayload {
    team_id: string;
}

export interface BoardMePayload {
    board_id: string;
    version: number;
    votes_remaining: number;
    my_votes: Record<string, number>;
}
