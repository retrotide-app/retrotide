import type { BoardRef } from "../../types/api";
import { sendRaw } from "./socket";

export const REALTIME_COMMANDS = {
    PING: "ping",
    BOARD_JOIN: "board_join",
    BOARD_LEAVE: "board_leave",
} as const satisfies Record<string, RealtimeCommandName>;

export interface PingPayload {
    nonce?: number;
    rtt?: number;
}

export type BoardJoinPayload = BoardRef;

export type BoardLeavePayload = BoardRef;

export type RealtimeCommand =
    | { type: "ping"; data: PingPayload }
    | { type: "board_join"; data: BoardJoinPayload }
    | { type: "board_leave"; data: BoardLeavePayload };

export type RealtimeCommandName = RealtimeCommand["type"];

export type RealtimeCommandOf<K extends RealtimeCommandName> = Extract<RealtimeCommand, { type: K }>;

export type RealtimeCommandPayload<K extends RealtimeCommandName> = RealtimeCommandOf<K>["data"];

export function sendRealtime(command: RealtimeCommand): void {
    sendRaw(JSON.stringify(command));
}
