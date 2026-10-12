import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeMutationState,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardInteractionSettings } from "./BoardInteractionSettings";

const { useUpdateBoardSettings, useResetVotes, useRevealCards, useHideCards } = vi.hoisted(() => ({
    useUpdateBoardSettings: vi.fn(),
    useResetVotes: vi.fn(),
    useRevealCards: vi.fn(),
    useHideCards: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => ({
    useUpdateBoardSettings,
    useResetVotes,
    useRevealCards,
    useHideCards,
}));

let update = makeMutationState();
let reset = makeMutationState();
let reveal = makeMutationState();
let hide = makeMutationState();

function renderSettings(snapshot: BoardSnapshot = makeBoardSnapshot()) {
    return renderWithProviders(<BoardInteractionSettings snapshot={snapshot} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    update = makeMutationState();
    reset = makeMutationState();
    reveal = makeMutationState();
    hide = makeMutationState();
    useUpdateBoardSettings.mockReturnValue(update);
    useResetVotes.mockReturnValue(reset);
    useRevealCards.mockReturnValue(reveal);
    useHideCards.mockReturnValue(hide);
});

describe("BoardInteractionSettings", () => {
    it("shows each switch as the board has it", () => {
        // when
        renderSettings(
            makeBoardSnapshot({
                settings: makeBoardSettings({ vote_counts_hidden: true, comments_enabled: false }),
            }),
        );

        // then
        expect(screen.getByRole("switch", { name: "Voting open" })).toHaveAttribute("aria-checked", "true");
        expect(screen.getByRole("switch", { name: "Hide vote totals" })).toHaveAttribute("aria-checked", "true");
        expect(screen.getByRole("switch", { name: "Comments" })).toHaveAttribute("aria-checked", "false");
        expect(screen.getByRole("switch", { name: "Merging" })).toHaveAttribute("aria-checked", "true");
    });

    it.each([
        ["Voting open", { voting_open: false }],
        ["Hide vote totals", { vote_counts_hidden: true }],
        ["Comments", { comments_enabled: false }],
        ["Merging", { merging_enabled: false }],
    ])("sends only the %s switch it flipped", async (name, change) => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        await user.click(screen.getByRole("switch", { name }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(change);
    });

    it("saves both vote limits together", async () => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        await user.clear(screen.getByLabelText("Votes per person"));
        await user.type(screen.getByLabelText("Votes per person"), "3");
        await user.clear(screen.getByLabelText("Votes per card"));
        await user.type(screen.getByLabelText("Votes per card"), "1");
        await user.click(screen.getByRole("button", { name: "Save limits" }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith({ max_votes: 3, max_votes_per_card: 1 });
    });

    it("refuses a limit outside 0 to 99 before sending it", async () => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        await user.clear(screen.getByLabelText("Votes per person"));
        await user.type(screen.getByLabelText("Votes per person"), "120");
        await user.click(screen.getByRole("button", { name: "Save limits" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a whole number from 0 to 99.");
        expect(update.mutate).not.toHaveBeenCalled();
    });

    it("sets the order everyone starts with", async () => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        const order = screen.getByRole("group", { name: "Card order everyone starts with" });
        await user.click(within(order).getByRole("button", { name: "Most votes" }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith({ sort_mode: "votes" });
    });

    it("resets every vote once the facilitator confirms", async () => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        await user.click(screen.getByRole("button", { name: "Reset votes" }));
        const dialog = screen.getByRole("dialog", { name: "Reset every vote?" });
        await user.click(within(dialog).getByRole("button", { name: "Reset votes" }));

        // then
        expect(reset.mutate).toHaveBeenCalledExactlyOnceWith(undefined, expect.anything());
    });

    it("shows a refused change", () => {
        // given
        useUpdateBoardSettings.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 403, message: "your role on this board does not allow that" }),
            }),
        );

        // when
        renderSettings();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("your role on this board does not allow that");
    });

    it("shows the locks and the timer sound as the board has them", () => {
        // when
        renderSettings(
            makeBoardSnapshot({
                settings: makeBoardSettings({ moves_locked: true, timer_play_sound: false }),
            }),
        );

        // then
        expect(screen.getByRole("switch", { name: "Lock cards" })).toHaveAttribute("aria-checked", "false");
        expect(screen.getByRole("switch", { name: "Lock moving" })).toHaveAttribute("aria-checked", "true");
        expect(screen.getByRole("switch", { name: "Lock the board" })).toHaveAttribute("aria-checked", "false");
        expect(screen.getByRole("switch", { name: "Timer sound" })).toHaveAttribute("aria-checked", "false");
    });

    it.each([
        ["Lock cards", { cards_locked: true }],
        ["Lock moving", { moves_locked: true }],
        ["Lock the board", { board_locked: true }],
        ["Timer sound", { timer_play_sound: false }],
    ])("sends only the %s switch it flipped", async (name, change) => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        await user.click(screen.getByRole("switch", { name }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(change);
    });

    it("shows auto-lock, sort enforcement and anyone can edit as the board has them", () => {
        // when
        renderSettings(
            makeBoardSnapshot({
                settings: makeBoardSettings({ timer_auto_lock: true, sort_enforced: false, anyone_can_edit: true }),
            }),
        );

        // then
        expect(screen.getByRole("switch", { name: "Lock the board when the timer ends" })).toHaveAttribute(
            "aria-checked",
            "true",
        );
        expect(screen.getByRole("switch", { name: "Enforce sort order" })).toHaveAttribute("aria-checked", "false");
        expect(screen.getByRole("switch", { name: "Enforce sort order" })).toHaveTextContent(
            "Everyone sees the board's sort order",
        );
        expect(screen.getByRole("switch", { name: "Anyone can edit" })).toHaveAttribute("aria-checked", "true");
        expect(screen.getByRole("switch", { name: "Anyone can edit" })).toHaveTextContent(
            "Participants can run the timer and add, edit and move columns",
        );
    });

    it.each([
        ["Lock the board when the timer ends", { timer_auto_lock: true }],
        ["Enforce sort order", { sort_enforced: true }],
        ["Anyone can edit", { anyone_can_edit: true }],
    ])("sends only the %s switch it flipped", async (name, change) => {
        // given
        const user = userEvent.setup();
        renderSettings();

        // when
        await user.click(screen.getByRole("switch", { name }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(change);
    });

    it("reveals the cards when the facilitator switches hiding off", async () => {
        // given
        const user = userEvent.setup();
        renderSettings(makeBoardSnapshot({ board: makeBoardView({ cards_hidden: true }) }));

        // when
        await user.click(screen.getByRole("switch", { name: "Cards hidden" }));

        // then
        expect(reveal.mutate).toHaveBeenCalledTimes(1);
        expect(hide.mutate).not.toHaveBeenCalled();
    });

    it("hides the cards again, saying it only blurs what people have already read", async () => {
        // given
        const user = userEvent.setup();
        renderSettings(makeBoardSnapshot({ board: makeBoardView({ cards_hidden: false }) }));

        // when
        const toggle = screen.getByRole("switch", { name: "Cards hidden" });
        await user.click(toggle);

        // then
        expect(toggle).toHaveTextContent("Anyone who has read a card has still read it.");
        expect(hide.mutate).toHaveBeenCalledTimes(1);
        expect(reveal.mutate).not.toHaveBeenCalled();
    });

    it.each(["vote", "discuss", "actions"] as const)(
        "leaves the Cards hidden switch out in %s, because cards are hidden only in Write",
        stage => {
            // when
            renderSettings(makeBoardSnapshot({ board: makeBoardView({ stage, cards_hidden: false }) }));

            // then
            expect(screen.queryByRole("switch", { name: "Cards hidden" })).not.toBeInTheDocument();
            expect(screen.getByRole("switch", { name: "Lock cards" })).toBeInTheDocument();
        },
    );

    it("turns card dates on and off with their own switch", async () => {
        // given
        const user = userEvent.setup();
        renderSettings(makeBoardSnapshot({ settings: makeBoardSettings({ show_card_dates: true }) }));
        const dates = screen.getByRole("switch", { name: "Show card dates" });

        // when
        await user.click(dates);

        // then
        expect(dates).toHaveAttribute("aria-checked", "true");
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith({ show_card_dates: false });
        expect(screen.getByText("Show on each card the date it was written.")).toBeInTheDocument();
    });

    it("says an anonymous board never shows card dates, even with the switch on", () => {
        // when
        renderSettings(makeBoardSnapshot({ board: makeBoardView({ anonymity: "anonymous" }) }));

        // then
        expect(screen.getByRole("switch", { name: "Show card dates" })).toHaveAttribute("aria-checked", "false");
        expect(
            screen.getByText("This board is anonymous, so cards never show when they were written, even with this on."),
        ).toBeInTheDocument();
    });

    it("shows a refused reveal", () => {
        // given
        useRevealCards.mockReturnValue(
            makeMutationState({ error: makeApiError({ status: 409, message: "read only", code: "read_only" }) }),
        );

        // when
        renderSettings();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("read only");
    });
});
