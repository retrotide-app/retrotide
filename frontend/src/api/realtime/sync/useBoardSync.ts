import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { applyBoardEvents } from "../../../domain/board/reduce";
import {
    receiveFrame,
    receiveJoined,
    receiveRefusal,
    type SequencerState,
    type SequencerStep,
    settleSequencer,
    startSequencer,
} from "../../../domain/board/sequencer";
import { type BoardSyncStatus, SYNC_CONNECTING, SYNC_LIVE } from "../../../domain/board/syncStatus";
import { withMe } from "../../../domain/board/votes";
import type { BoardEvent, BoardRef, BoardRefusalReason, BoardSnapshot } from "../../../types/api";
import { catchUpBoard, refreshPreviousActions } from "../../cache/boardSnapshot";
import { queryKeys } from "../../queryKeys";
import { reportClientError } from "../../telemetry";
import { REALTIME_EVENTS, type RealtimeEventName } from "../events";
import { REALTIME_COMMANDS, sendRealtime } from "../outbound";
import { getRealtimeConnected, subscribeRealtimeEpoch } from "../pipeline";
import { useRealtimeEvent, useRealtimeStatus } from "../useRealtime";

interface ReportedStatus {
    epoch: number;
    boardId: string;
    status: BoardSyncStatus;
}

interface SyncSession {
    active: boolean;
    catchUp: ReturnType<typeof setTimeout> | null;
}

const JOIN_RETRY_MS = 5_000;

const CATCH_UP_RETRY_MS = 5_000;

const IMMEDIATE_CATCH_UPS = 2;

type LiveBoardEventName = Extract<
    RealtimeEventName,
    "board_joined" | "board_join_rejected" | "board_access_revoked" | "board_events" | "board_me"
>;

const LIVE_BOARD_EVENTS: readonly LiveBoardEventName[] = [
    REALTIME_EVENTS.BOARD_JOINED,
    REALTIME_EVENTS.BOARD_JOIN_REJECTED,
    REALTIME_EVENTS.BOARD_ACCESS_REVOKED,
    REALTIME_EVENTS.BOARD_EVENTS,
    REALTIME_EVENTS.BOARD_ME,
];

export function useBoardSync(boardId: string): BoardSyncStatus {
    const qc = useQueryClient();
    const epoch = useRealtimeStatus();
    const connected = useSyncExternalStore(subscribeRealtimeEpoch, getRealtimeConnected);
    const [reported, setReported] = useState<ReportedStatus>({ epoch: 0, boardId, status: SYNC_CONNECTING });

    const sequencerRef = useRef<SequencerState<BoardEvent>>(startSequencer<BoardEvent>());
    const epochRef = useRef(0);
    const fetchingRef = useRef(false);
    const catchUpsRef = useRef(0);
    const sessionRef = useRef<SyncSession>({ active: false, catchUp: null });

    function appliedVersion(): number | null {
        return qc.getQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId))?.version ?? null;
    }

    function stillWanted(session: SyncSession): boolean {
        return session.active && sessionRef.current === session && sequencerRef.current.phase !== "refused";
    }

    function startCatchUp(session: SyncSession): void {
        fetchingRef.current = true;
        catchUpsRef.current += 1;

        catchUpBoard(qc, boardId).then(
            () => {
                if (!stillWanted(session)) {
                    return;
                }

                fetchingRef.current = false;
                run(settleSequencer(sequencerRef.current, appliedVersion()));
            },
            () => {
                if (!stillWanted(session)) {
                    return;
                }

                fetchingRef.current = false;
                requestCatchUp();
            },
        );
    }

    function requestCatchUp(): void {
        const session = sessionRef.current;

        if (!stillWanted(session) || fetchingRef.current || session.catchUp !== null) {
            return;
        }

        if (catchUpsRef.current < IMMEDIATE_CATCH_UPS) {
            startCatchUp(session);
            return;
        }

        session.catchUp = setTimeout(() => {
            session.catchUp = null;

            if (!stillWanted(session)) {
                return;
            }

            startCatchUp(session);
        }, CATCH_UP_RETRY_MS);
    }

    function run(step: SequencerStep<BoardEvent>): void {
        if (step.state.phase === "refused") {
            return;
        }

        sequencerRef.current = step.state;

        const { apply } = step;
        if (apply !== null) {
            qc.setQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId), held =>
                held === undefined ? held : applyBoardEvents(held, apply.events, apply.to),
            );
        }

        const live = step.state.phase === "live";
        if (live) {
            catchUpsRef.current = 0;
        }

        setReported({ epoch: epochRef.current, boardId, status: live ? SYNC_LIVE : SYNC_CONNECTING });

        if (step.refetch) {
            requestCatchUp();
        }
    }

    function refuse(reason: BoardRefusalReason, revoked: boolean): void {
        sequencerRef.current = receiveRefusal<BoardEvent>();

        const session = sessionRef.current;
        if (session.catchUp !== null) {
            clearTimeout(session.catchUp);
            session.catchUp = null;
        }

        setReported({ epoch: epochRef.current, boardId, status: { state: "refused", reason, revoked } });
    }

    useRealtimeEvent(LIVE_BOARD_EVENTS, event => {
        if (event.data.board_id !== boardId) {
            return;
        }

        switch (event.type) {
            case "board_events":
                run(receiveFrame(sequencerRef.current, appliedVersion(), event.data));
                return;
            case "board_joined":
                run(receiveJoined(sequencerRef.current, appliedVersion(), event.data.version));
                return;
            case "board_join_rejected":
                refuse(event.data.reason, false);
                return;
            case "board_access_revoked":
                refuse(event.data.reason, true);
                return;
            case "board_me": {
                const { version, votes_remaining: votesRemaining, my_votes: myVotes } = event.data;

                qc.setQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId), held =>
                    held === undefined
                        ? held
                        : withMe(held, { votes_remaining: votesRemaining, my_votes: myVotes }, version),
                );
            }
        }
    });

    useRealtimeEvent(REALTIME_EVENTS.TEAM_ACTION_ITEMS_CHANGED, event => {
        const held = qc.getQueryData<BoardSnapshot>(queryKeys.boardLive.state(boardId));

        if (held === undefined || held.board.team_id !== event.data.team_id) {
            return;
        }

        refreshPreviousActions(qc, boardId).catch((error: unknown) => {
            reportClientError(error, { source: "board-refresh" });
        });
    });

    useEffect(() => {
        if (epoch === 0) {
            return;
        }

        const session: SyncSession = { active: true, catchUp: null };
        sessionRef.current = session;
        epochRef.current = epoch;
        sequencerRef.current = startSequencer<BoardEvent>();
        fetchingRef.current = false;
        catchUpsRef.current = 0;

        const board: BoardRef = { board_id: boardId };
        let joinRetry: ReturnType<typeof setTimeout> | undefined;

        const join = () => {
            sendRealtime({ type: REALTIME_COMMANDS.BOARD_JOIN, data: board });

            joinRetry = setTimeout(() => {
                if (sequencerRef.current.phase === "joining") {
                    join();
                }
            }, JOIN_RETRY_MS);
        };

        join();

        return () => {
            session.active = false;
            clearTimeout(joinRetry);

            if (session.catchUp !== null) {
                clearTimeout(session.catchUp);
                session.catchUp = null;
            }

            sendRealtime({ type: REALTIME_COMMANDS.BOARD_LEAVE, data: board });
        };
    }, [boardId, epoch]);

    if (epoch === 0 || reported.epoch !== epoch || reported.boardId !== boardId) {
        return SYNC_CONNECTING;
    }

    if (!connected && reported.status.state !== "refused") {
        return SYNC_CONNECTING;
    }

    return reported.status;
}
