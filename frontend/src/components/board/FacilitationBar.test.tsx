import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeMutationState,
    makeParticipantViewer,
    makeTimer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, BoardStage } from "../../types/api";
import { FacilitationBar } from "./FacilitationBar";

const mutations = vi.hoisted(() => ({
    useChangeStage: vi.fn(),
    useCloseBoard: vi.fn(),
    useBoardTimerCommand: vi.fn(),
    useUpdateBoardSettings: vi.fn(),
    useSetParticipantRole: vi.fn(),
    useRemoveParticipant: vi.fn(),
}));

const { useBoardTimer } = vi.hoisted(() => ({ useBoardTimer: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => mutations);
vi.mock("../../hooks/board/useBoardTimer", () => ({ useBoardTimer }));

function renderBar(snapshot: BoardSnapshot = makeBoardSnapshot(), writable = true) {
    return renderWithProviders(<FacilitationBar snapshot={snapshot} writable={writable} />);
}

function inStage(stage: BoardStage, overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ stage, cards_hidden: false }),
        hidden_counts: {},
        ...overrides,
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    useBoardTimer.mockReturnValue(null);
});

describe("FacilitationBar", () => {
    it("offers a facilitator the next stage, the timer and the end of the retro, and no reveal or hide", () => {
        // when
        renderBar();

        // then
        const bar = screen.getByRole("region", { name: "Facilitation" });
        expect(within(bar).getByRole("button", { name: "Next stage: Vote" })).toBeInTheDocument();
        expect(within(bar).getByRole("combobox", { name: "Timer length" })).toBeInTheDocument();
        expect(within(bar).getByRole("button", { name: "End retro" })).toBeInTheDocument();
        expect(within(bar).queryByRole("button", { name: "Reveal cards" })).not.toBeInTheDocument();
        expect(within(bar).queryByRole("button", { name: "Hide cards" })).not.toBeInTheDocument();
    });

    it("names the stage after the current one", () => {
        // when
        renderBar(inStage("discuss"));

        // then
        expect(screen.getByRole("button", { name: "Next stage: Actions" })).toBeInTheDocument();
    });

    it("asks before moving on to Vote when that reveals the hidden cards, then names Vote", async () => {
        // given
        const advance = makeMutationState();
        mutations.useChangeStage.mockReturnValue(advance);
        const user = userEvent.setup();
        renderBar();

        // when
        await user.click(screen.getByRole("button", { name: "Next stage: Vote" }));
        const dialog = screen.getByRole("dialog", { name: "Reveal the cards and start voting?" });

        // then
        expect(advance.mutate).not.toHaveBeenCalled();
        expect(dialog).toHaveTextContent("Cards cannot be hidden again after this.");

        // when
        await user.click(within(dialog).getByRole("button", { name: "Reveal cards" }));

        // then
        expect(advance.mutate).toHaveBeenCalledTimes(1);
        expect(advance.mutate).toHaveBeenCalledWith({ stage: "vote" }, expect.anything());
    });

    it("says an anonymous board's cards are shuffled as they are revealed", async () => {
        // given
        const user = userEvent.setup();
        renderBar(makeBoardSnapshot({ board: makeBoardView({ anonymity: "anonymous" }) }));

        // when
        await user.click(screen.getByRole("button", { name: "Next stage: Vote" }));

        // then
        expect(screen.getByRole("dialog", { name: "Reveal the cards and start voting?" })).toHaveTextContent(
            "the cards in each column are shuffled",
        );
    });

    it("moves on to Vote at once when the cards were already shown", async () => {
        // given
        const advance = makeMutationState();
        mutations.useChangeStage.mockReturnValue(advance);
        const user = userEvent.setup();
        renderBar(inStage("write"));

        // when
        await user.click(screen.getByRole("button", { name: "Next stage: Vote" }));

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(advance.mutate).toHaveBeenCalledWith({ stage: "vote" });
    });

    it.each([
        ["vote", "Discuss", "discuss"],
        ["discuss", "Actions", "actions"],
    ] as const)("moves on from %s to %s without asking", async (stage, label, next) => {
        // given
        const advance = makeMutationState();
        mutations.useChangeStage.mockReturnValue(advance);
        const user = userEvent.setup();
        renderBar(inStage(stage));

        // when
        await user.click(screen.getByRole("button", { name: `Next stage: ${label}` }));

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(advance.mutate).toHaveBeenCalledWith({ stage: next });
    });

    it("ends the retro from Actions through the stage, behind the end warning, with no separate End retro", async () => {
        // given
        const advance = makeMutationState();
        const close = makeMutationState();
        mutations.useChangeStage.mockReturnValue(advance);
        mutations.useCloseBoard.mockReturnValue(close);
        const user = userEvent.setup();
        renderBar(
            inStage("actions", {
                board: makeBoardView({ stage: "actions", cards_hidden: false, anonymity: "anonymous" }),
            }),
        );

        // then
        expect(screen.queryByRole("button", { name: "End retro" })).not.toBeInTheDocument();

        // when
        await user.click(screen.getByRole("button", { name: "Next stage: End" }));
        const dialog = screen.getByRole("dialog", { name: "End this retro?" });
        await user.click(within(dialog).getByRole("button", { name: "End retro" }));

        // then
        expect(dialog).toHaveTextContent("the link between people and their cards and votes is deleted for good");
        expect(advance.mutate).toHaveBeenCalledWith({ stage: "end" }, expect.anything());
        expect(close.mutate).not.toHaveBeenCalled();
    });

    it.each([
        ["vote", "Write", "write"],
        ["discuss", "Vote", "vote"],
        ["actions", "Discuss", "discuss"],
    ] as const)("steps back from %s to %s at once, without asking", async (stage, label, previous) => {
        // given
        const advance = makeMutationState();
        mutations.useChangeStage.mockReturnValue(advance);
        const user = userEvent.setup();
        renderBar(inStage(stage));

        // when
        await user.click(screen.getByRole("button", { name: `Back to ${label}` }));

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(advance.mutate).toHaveBeenCalledWith({ stage: previous });
    });

    it("offers no way back from Write", () => {
        // when
        renderBar(inStage("write"));

        // then
        expect(screen.queryByRole("button", { name: /^Back/ })).not.toBeInTheDocument();
    });

    it("offers a participant no way back", () => {
        // when
        renderBar(inStage("discuss", { viewer: makeParticipantViewer(), timer: makeTimer() }));

        // then
        expect(screen.queryByRole("button", { name: /^Back/ })).not.toBeInTheDocument();
    });

    it("holds the way back while a stage change is on its way", () => {
        // given
        mutations.useChangeStage.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderBar(inStage("discuss"));

        // then
        expect(screen.getByRole("button", { name: "Back to Vote" })).toBeDisabled();
    });

    it("warns that ending an anonymous retro deletes who wrote what", async () => {
        // given
        const user = userEvent.setup();
        renderBar(makeBoardSnapshot({ board: makeBoardView({ anonymity: "anonymous" }) }));

        // when
        await user.click(screen.getByRole("button", { name: "End retro" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "End this retro?" });
        expect(dialog).toHaveTextContent("the link between people and their cards and votes is deleted for good");
        expect(dialog).toHaveTextContent("This cannot be undone.");
    });

    it("leaves the anonymity warning off a named board's end", async () => {
        // given
        const user = userEvent.setup();
        renderBar();

        // when
        await user.click(screen.getByRole("button", { name: "End retro" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "End this retro?" });
        expect(dialog).not.toHaveTextContent("deleted for good");
    });

    it("ends the retro once confirmed, and shows why it failed if it did", async () => {
        // given
        const close = makeMutationState({
            error: makeApiError({ status: 503, message: "The board is busy", code: "board_busy" }),
        });
        mutations.useCloseBoard.mockReturnValue(close);
        const user = userEvent.setup();
        renderBar();

        // when
        await user.click(screen.getByRole("button", { name: "End retro" }));
        const dialog = screen.getByRole("dialog", { name: "End this retro?" });
        await user.click(within(dialog).getByRole("button", { name: "End retro" }));

        // then
        expect(close.mutate).toHaveBeenCalledTimes(1);
        expect(within(dialog).getByRole("alert")).toHaveTextContent("The board is busy");
    });

    it("shows a facilitator each lock that is on", () => {
        // when
        renderBar(makeBoardSnapshot({ settings: makeBoardSettings({ moves_locked: true, board_locked: true }) }));

        // then
        expect(screen.getByText("Board locked")).toBeInTheDocument();
        expect(screen.getByText("Moving locked")).toBeInTheDocument();
        expect(screen.queryByText("Cards locked")).not.toBeInTheDocument();
    });

    it("tells a participant plainly what the locks stop them doing", () => {
        // when
        renderBar(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                settings: makeBoardSettings({ moves_locked: true }),
            }),
        );

        // then
        expect(screen.getByRole("status", { name: "Locks" })).toHaveTextContent(
            "Moving cards is locked, so participants cannot move or merge cards.",
        );
        expect(screen.queryByRole("button", { name: /Next stage/ })).not.toBeInTheDocument();
    });

    it("shows a participant the timer and nothing else", () => {
        // given
        const timer = makeTimer();
        useBoardTimer.mockReturnValue({ timer, reading: { remainingMs: 30_000, phase: "running", tone: "amber" } });

        // when
        renderBar(makeBoardSnapshot({ viewer: makeParticipantViewer(), timer }));

        // then
        expect(screen.getByRole("timer", { name: "Time left" })).toHaveTextContent("0:30");
        expect(screen.queryByRole("button", { name: "Pause" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "End retro" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: /Next stage/ })).not.toBeInTheDocument();
    });

    it("lets a participant run the timer when anyone can edit, with nothing else a facilitator has", () => {
        // when
        renderBar(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                settings: makeBoardSettings({ anyone_can_edit: true }),
            }),
        );

        // then
        const bar = screen.getByRole("region", { name: "Facilitation" });
        expect(within(bar).getByRole("button", { name: "Start timer" })).toBeInTheDocument();
        expect(within(bar).queryByRole("button", { name: "Timer sound" })).not.toBeInTheDocument();
        expect(within(bar).queryByRole("button", { name: "Lock at time up" })).not.toBeInTheDocument();
        expect(within(bar).queryByRole("button", { name: /Next stage/ })).not.toBeInTheDocument();
        expect(within(bar).queryByRole("button", { name: "End retro" })).not.toBeInTheDocument();
    });

    it("gives a participant pause and stop on a running timer when anyone can edit", () => {
        // given
        const timer = makeTimer();
        useBoardTimer.mockReturnValue({ timer, reading: { remainingMs: 30_000, phase: "running", tone: "amber" } });

        // when
        renderBar(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                settings: makeBoardSettings({ anyone_can_edit: true }),
                timer,
            }),
        );

        // then
        expect(screen.getByRole("button", { name: "Pause" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Stop" })).toBeInTheDocument();
    });

    it("gives a participant no timer controls when anyone can edit but they have not joined", () => {
        // when
        renderBar(
            makeBoardSnapshot({
                viewer: makeParticipantViewer(),
                settings: makeBoardSettings({ anyone_can_edit: true }),
            }),
            false,
        );

        // then
        expect(screen.queryByRole("region", { name: "Facilitation" })).not.toBeInTheDocument();
    });

    it("offers a facilitator the lock at time up switch beside the timer sound", async () => {
        // given
        const update = makeMutationState();
        mutations.useUpdateBoardSettings.mockReturnValue(update);
        const user = userEvent.setup();
        renderBar();

        // when
        await user.click(screen.getByRole("button", { name: "Lock at time up" }));

        // then
        expect(screen.getByRole("button", { name: "Timer sound" })).toBeInTheDocument();
        expect(update.mutate).toHaveBeenCalledWith({ timer_auto_lock: true });
    });

    it("says a running timer locks the board when time is up", () => {
        // given
        const timer = makeTimer();
        useBoardTimer.mockReturnValue({ timer, reading: { remainingMs: 30_000, phase: "running", tone: "amber" } });

        // when
        renderBar(makeBoardSnapshot({ settings: makeBoardSettings({ timer_auto_lock: true }), timer }));

        // then
        expect(screen.getByText("Locks when time is up")).toBeInTheDocument();
    });

    it("shows nothing to a participant when there is no timer and no lock", () => {
        // when
        renderBar(makeBoardSnapshot({ viewer: makeParticipantViewer() }));

        // then
        expect(screen.queryByRole("region", { name: "Facilitation" })).not.toBeInTheDocument();
    });

    it("offers nothing until the viewer has joined the board", () => {
        // when
        renderBar(makeBoardSnapshot(), false);

        // then
        expect(screen.queryByRole("region", { name: "Facilitation" })).not.toBeInTheDocument();
    });

    it("shows the server's reason when a stage change is refused", () => {
        // given
        mutations.useChangeStage.mockReturnValue(
            makeMutationState({
                error: makeApiError({
                    status: 409,
                    message:
                        "a board moves one stage at a time, so it can only go to the stage straight after its current one or back to the one before it",
                    code: "stage_not_next",
                }),
            }),
        );

        // when
        renderBar(inStage("vote"));

        // then
        expect(screen.getByRole("status")).toHaveTextContent("a board moves one stage at a time");
    });
});
