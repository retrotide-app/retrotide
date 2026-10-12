import type {
    BoardEventsFrame,
    BoardJoinedPayload,
    BoardMePayload,
    BoardPresencePayload,
    BoardRefusalPayload,
    TeamActionItemsChangedPayload,
} from "../../types/api";

export const REALTIME_EVENTS = {
    PONG: "pong",
    ERROR: "error",
    BOARD_JOINED: "board_joined",
    BOARD_JOIN_REJECTED: "board_join_rejected",
    BOARD_ACCESS_REVOKED: "board_access_revoked",
    BOARD_PRESENCE: "board_presence",
    BOARD_EVENTS: "board_events",
    BOARD_ME: "board_me",
    TEAM_ACTION_ITEMS_CHANGED: "team_action_items_changed",
} as const satisfies Record<string, RealtimeEventName>;

export interface PongPayload {
    nonce?: number;
    server_now?: string;
}

export interface ErrorPayload {
    error: string;
}

export type RealtimeEvent =
    | { type: "pong"; data: PongPayload }
    | { type: "error"; data: ErrorPayload }
    | { type: "board_joined"; data: BoardJoinedPayload }
    | { type: "board_join_rejected"; data: BoardRefusalPayload }
    | { type: "board_access_revoked"; data: BoardRefusalPayload }
    | { type: "board_presence"; data: BoardPresencePayload }
    | { type: "board_events"; data: BoardEventsFrame }
    | { type: "board_me"; data: BoardMePayload }
    | { type: "team_action_items_changed"; data: TeamActionItemsChangedPayload };

export type RealtimeEventName = RealtimeEvent["type"];

export type RealtimeEventOf<K extends RealtimeEventName> = Extract<RealtimeEvent, { type: K }>;

export type RealtimeEventPayload<K extends RealtimeEventName> = RealtimeEventOf<K>["data"];
