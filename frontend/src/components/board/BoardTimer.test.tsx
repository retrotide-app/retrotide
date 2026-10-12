import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TimerReading } from "../../domain/board/timer";
import {
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeMutationState,
    makeTimer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, TimerView } from "../../types/api";
import { BoardTimer } from "./BoardTimer";

const { useBoardTimerCommand, useUpdateBoardSettings } = vi.hoisted(() => ({
    useBoardTimerCommand: vi.fn(),
    useUpdateBoardSettings: vi.fn(),
}));

const { useBoardTimer } = vi.hoisted(() => ({ useBoardTimer: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useBoardTimerCommand, useUpdateBoardSettings }));
vi.mock("../../hooks/board/useBoardTimer", () => ({ useBoardTimer }));

let command = makeMutationState();

let settings = makeMutationState();

function showTimer(reading: TimerReading, timer: TimerView = makeTimer()) {
    useBoardTimer.mockReturnValue({ timer, reading });
}

interface TimerRender {
    controls?: boolean;
    facilitates?: boolean;
    snapshot?: BoardSnapshot;
}

function renderTimer({ controls = true, facilitates = controls, snapshot = makeBoardSnapshot() }: TimerRender = {}) {
    return renderWithProviders(<BoardTimer snapshot={snapshot} controls={controls} facilitates={facilitates} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    command = makeMutationState();
    settings = makeMutationState();
    useBoardTimerCommand.mockReturnValue(command);
    useUpdateBoardSettings.mockReturnValue(settings);
    useBoardTimer.mockReturnValue(null);
});

describe("BoardTimer", () => {
    it("starts a timer of the length the facilitator picked", async () => {
        // given
        const user = userEvent.setup();
        renderTimer();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Timer length" }), "10");
        await user.click(screen.getByRole("button", { name: "Start timer" }));

        // then
        expect(command.mutate).toHaveBeenCalledWith({ action: "start", duration_ms: 600_000 });
    });

    it("starts five minutes unless told otherwise", async () => {
        // given
        const user = userEvent.setup();
        renderTimer();

        // when
        await user.click(screen.getByRole("button", { name: "Start timer" }));

        // then
        expect(command.mutate).toHaveBeenCalledWith({ action: "start", duration_ms: 300_000 });
    });

    it("shows nothing to a participant while no timer is set", () => {
        // when
        renderTimer({ controls: false });

        // then
        expect(screen.queryByRole("group", { name: "Timer" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });

    it("shows a running timer's time left and lets the facilitator pause it, add a minute or stop it", async () => {
        // given
        showTimer({ remainingMs: 125_000, phase: "running", tone: "calm" });
        const user = userEvent.setup();
        renderTimer();

        // when
        await user.click(screen.getByRole("button", { name: "Pause" }));
        await user.click(screen.getByRole("button", { name: "+1 minute" }));
        await user.click(screen.getByRole("button", { name: "Stop" }));

        // then
        expect(screen.getByRole("timer", { name: "Time left" })).toHaveTextContent("2:05");
        expect(command.mutate.mock.calls.map(([change]) => change)).toEqual([
            { action: "pause" },
            { action: "add_minute" },
            { action: "stop" },
        ]);
    });

    it("says a paused timer is paused and lets the facilitator resume it", async () => {
        // given
        showTimer(
            { remainingMs: 42_000, phase: "paused", tone: "amber" },
            makeTimer({ ends_at: null, paused_remaining_ms: 42_000 }),
        );
        const user = userEvent.setup();
        renderTimer();

        // when
        await user.click(screen.getByRole("button", { name: "Resume" }));

        // then
        expect(screen.getByText("Paused")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Pause" })).not.toBeInTheDocument();
        expect(command.mutate).toHaveBeenCalledWith({ action: "resume" });
    });

    it("says when time is up, and still offers another minute", () => {
        // given
        showTimer({ remainingMs: 0, phase: "finished", tone: "red" }, makeTimer({ duration_ms: 3_600_000 }));

        // when
        renderTimer();

        // then
        expect(screen.getByRole("timer", { name: "Time left" })).toHaveTextContent("0:00");
        expect(screen.getByText("Time is up")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "+1 minute" })).toBeEnabled();
        expect(screen.queryByRole("button", { name: "Pause" })).not.toBeInTheDocument();
    });

    it("refuses another minute that would take a running timer past an hour", () => {
        // given
        showTimer({ remainingMs: 30_000, phase: "running", tone: "amber" }, makeTimer({ duration_ms: 3_570_000 }));

        // when
        renderTimer();

        // then
        expect(screen.getByRole("button", { name: "+1 minute" })).toBeDisabled();
    });

    it("shows a participant the timer with no controls", () => {
        // given
        showTimer({ remainingMs: 61_000, phase: "running", tone: "calm" });

        // when
        renderTimer({ controls: false });

        // then
        expect(screen.getByRole("timer", { name: "Time left" })).toHaveTextContent("1:01");
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });

    it("switches the board's timer sound", async () => {
        // given
        const user = userEvent.setup();
        renderTimer({ snapshot: makeBoardSnapshot({ settings: makeBoardSettings({ timer_play_sound: true }) }) });

        // when
        const toggle = screen.getByRole("button", { name: "Timer sound" });
        await user.click(toggle);

        // then
        expect(toggle).toHaveAttribute("aria-pressed", "true");
        expect(settings.mutate).toHaveBeenCalledWith({ timer_play_sound: false });
    });

    it("switches the board's lock at time up beside the timer sound", async () => {
        // given
        const user = userEvent.setup();
        renderTimer({ snapshot: makeBoardSnapshot({ settings: makeBoardSettings({ timer_auto_lock: false }) }) });

        // when
        const toggle = screen.getByRole("button", { name: "Lock at time up" });
        await user.click(toggle);

        // then
        expect(toggle).toHaveAttribute("aria-pressed", "false");
        expect(settings.mutate).toHaveBeenCalledWith({ timer_auto_lock: true });
    });

    it("lets a participant who may run the timer start it, without the facilitator's switches", async () => {
        // given
        const user = userEvent.setup();
        renderTimer({ controls: true, facilitates: false });

        // when
        await user.click(screen.getByRole("button", { name: "Start timer" }));

        // then
        expect(command.mutate).toHaveBeenCalledWith({ action: "start", duration_ms: 300_000 });
        expect(screen.queryByRole("button", { name: "Timer sound" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Lock at time up" })).not.toBeInTheDocument();
    });

    it("lets a participant who may run the timer pause a running one", async () => {
        // given
        showTimer({ remainingMs: 61_000, phase: "running", tone: "calm" });
        const user = userEvent.setup();
        renderTimer({ controls: true, facilitates: false });

        // when
        await user.click(screen.getByRole("button", { name: "Pause" }));

        // then
        expect(command.mutate).toHaveBeenCalledWith({ action: "pause" });
        expect(screen.queryByRole("button", { name: "Timer sound" })).not.toBeInTheDocument();
    });

    it("tells everyone a running timer will lock the board", () => {
        // given
        showTimer({ remainingMs: 61_000, phase: "running", tone: "calm" });

        // when
        renderTimer({
            controls: false,
            snapshot: makeBoardSnapshot({ settings: makeBoardSettings({ timer_auto_lock: true }) }),
        });

        // then
        expect(screen.getByText("Locks when time is up")).toBeInTheDocument();
    });

    it.each([
        ["auto-lock is off", { remainingMs: 61_000, phase: "running", tone: "calm" } as const, false],
        ["the timer is paused", { remainingMs: 61_000, phase: "paused", tone: "calm" } as const, true],
    ])("says nothing about locking while %s", (_, reading, autoLock) => {
        // given
        showTimer(reading);

        // when
        renderTimer({ snapshot: makeBoardSnapshot({ settings: makeBoardSettings({ timer_auto_lock: autoLock }) }) });

        // then
        expect(screen.queryByText("Locks when time is up")).not.toBeInTheDocument();
    });

    it("explains a refused timer command", () => {
        // given
        showTimer({ remainingMs: 30_000, phase: "running", tone: "amber" });
        useBoardTimerCommand.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "timer is too long", code: "timer_too_long" }),
            }),
        );

        // when
        renderTimer();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("A timer can run for an hour at most.");
    });
});
