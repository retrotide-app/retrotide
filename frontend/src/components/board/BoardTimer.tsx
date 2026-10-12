import { useState } from "react";
import { facilitationRefusal } from "../../domain/board/facilitation";
import {
    canAddMinute,
    DEFAULT_TIMER_MINUTES,
    formatTimer,
    TIMER_LENGTHS_MINUTES,
    timerLengthLabel,
    timerLengthMs,
} from "../../domain/board/timer";
import { useBoardTimer } from "../../hooks/board/useBoardTimer";
import { useBoardTimerCommand, useUpdateBoardSettings } from "../../hooks/mutations/boards";
import type { BoardSnapshot } from "../../types/api";
import { Button } from "../Button/Button";
import { Select } from "../Select/Select";
import { StatusTag } from "../StatusTag/StatusTag";
import { Toast } from "../Toast/Toast";
import styles from "./BoardTimer.module.css";

interface BoardTimerProps {
    snapshot: BoardSnapshot;
    controls: boolean;
    facilitates: boolean;
}

export function BoardTimer({ snapshot, controls, facilitates }: BoardTimerProps) {
    const shown = useBoardTimer(snapshot);
    const command = useBoardTimerCommand(snapshot.board_id);
    const update = useUpdateBoardSettings(snapshot.board_id);
    const [minutes, setMinutes] = useState(DEFAULT_TIMER_MINUTES);
    const { timer_play_sound: playsSound, timer_auto_lock: autoLocks } = snapshot.settings;
    const busy = command.isPending;
    const failure = command.error ?? update.error;

    const settingToggles = facilitates && (
        <>
            <Button
                type="button"
                size="small"
                variant={playsSound ? "primary" : "secondary"}
                aria-pressed={playsSound}
                disabled={update.isPending}
                onClick={() => update.mutate({ timer_play_sound: !playsSound })}
            >
                Timer sound
            </Button>
            <Button
                type="button"
                size="small"
                variant={autoLocks ? "primary" : "secondary"}
                aria-pressed={autoLocks}
                disabled={update.isPending}
                onClick={() => update.mutate({ timer_auto_lock: !autoLocks })}
            >
                Lock at time up
            </Button>
        </>
    );

    const failureToast = failure !== null && (
        <Toast
            variant="error"
            onDismiss={() => {
                command.reset();
                update.reset();
            }}
        >
            {facilitationRefusal(failure)}
        </Toast>
    );

    if (shown === null) {
        if (!controls) {
            return null;
        }

        return (
            <div className={styles.timer} role="group" aria-label="Timer">
                <Select
                    size="small"
                    aria-label="Timer length"
                    className={styles.length}
                    value={String(minutes)}
                    onChange={e => setMinutes(Number(e.target.value))}
                >
                    {TIMER_LENGTHS_MINUTES.map(length => (
                        <option key={length} value={String(length)}>
                            {timerLengthLabel(length)}
                        </option>
                    ))}
                </Select>
                <Button
                    type="button"
                    size="small"
                    disabled={busy}
                    onClick={() => command.mutate({ action: "start", duration_ms: timerLengthMs(minutes) })}
                >
                    Start timer
                </Button>
                {settingToggles}
                {failureToast}
            </div>
        );
    }

    const { timer, reading } = shown;
    const paused = reading.phase === "paused";
    const running = reading.phase === "running";
    const finished = reading.phase === "finished";

    return (
        <div className={styles.timer} role="group" aria-label="Timer">
            <span className={`${styles.clock} ${styles[reading.tone]}`} role="timer" aria-label="Time left">
                {formatTimer(reading.remainingMs)}
            </span>
            {paused && <StatusTag tone="neutral">Paused</StatusTag>}
            {finished && <StatusTag tone="danger">Time is up</StatusTag>}
            {running && autoLocks && <StatusTag tone="warning">Locks when time is up</StatusTag>}
            {controls && (
                <>
                    {paused && (
                        <Button
                            type="button"
                            size="small"
                            disabled={busy}
                            onClick={() => command.mutate({ action: "resume" })}
                        >
                            Resume
                        </Button>
                    )}
                    {running && (
                        <Button
                            type="button"
                            size="small"
                            disabled={busy}
                            onClick={() => command.mutate({ action: "pause" })}
                        >
                            Pause
                        </Button>
                    )}
                    <Button
                        type="button"
                        size="small"
                        disabled={busy || !canAddMinute(timer, reading)}
                        onClick={() => command.mutate({ action: "add_minute" })}
                    >
                        +1 minute
                    </Button>
                    <Button
                        type="button"
                        size="small"
                        disabled={busy}
                        onClick={() => command.mutate({ action: "stop" })}
                    >
                        Stop
                    </Button>
                </>
            )}
            {settingToggles}
            {failureToast}
        </div>
    );
}
