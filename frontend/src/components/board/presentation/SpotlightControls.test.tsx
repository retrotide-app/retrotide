import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeColumn,
    makeMutationState,
    makeParticipantViewer,
    makeSpotlight,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BoardSnapshot } from "../../../types/api";
import { SpotlightControls } from "./SpotlightControls";

const hooks = vi.hoisted(() => ({
    usePresentCard: vi.fn(),
    useFocusColumn: vi.fn(),
    useHighlightCard: vi.fn(),
}));

vi.mock("../../../hooks/mutations/facilitation", () => hooks);

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const CONTINUE = "00000000-0000-4000-8000-000000000c03";

const discussing = makeBoardView({ stage: "discuss", cards_hidden: false });

const shown = makeBoardSnapshot({
    board: discussing,
    hidden_counts: {},
    cards: [
        makeCard({ id: "late", column_id: STOP, position: "a0", hidden: false }),
        makeCard({ id: "first", column_id: START, position: "a0", hidden: false }),
    ],
});

function renderControls(snapshot: BoardSnapshot = shown, writable = true) {
    return renderWithProviders(<SpotlightControls snapshot={snapshot} writable={writable} />);
}

let present = makeMutationState();
let focus = makeMutationState();
let highlight = makeMutationState();

beforeEach(() => {
    vi.clearAllMocks();
    present = makeMutationState();
    focus = makeMutationState();
    highlight = makeMutationState();
    hooks.usePresentCard.mockReturnValue(present);
    hooks.useFocusColumn.mockReturnValue(focus);
    hooks.useHighlightCard.mockReturnValue(highlight);
});

describe("SpotlightControls", () => {
    it("starts the presentation at the first card of the first column", async () => {
        // given
        const user = userEvent.setup();
        renderControls();

        // when
        await user.click(screen.getByRole("button", { name: "Present" }));

        // then
        expect(present.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "first" });
        expect(screen.queryByText("Reveal some cards first")).not.toBeInTheDocument();
    });

    it("keeps Present waiting while no card is shown to everyone", () => {
        // when
        renderControls(makeBoardSnapshot());

        // then
        expect(screen.getByRole("button", { name: "Present" })).toBeDisabled();
        expect(screen.getByText("Reveal some cards first")).toBeInTheDocument();
    });

    it("offers no Present button while a presentation is already running", () => {
        // when
        renderControls({ ...shown, spotlight: makeSpotlight({ presenting_card_id: "first" }) });

        // then
        expect(screen.queryByRole("button", { name: "Present" })).not.toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Focus column" })).toBeInTheDocument();
    });

    it("focuses a column for everyone", async () => {
        // given
        const user = userEvent.setup();
        renderControls();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Focus column" }), "Stop");

        // then
        expect(focus.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: STOP });
    });

    it("clears the focus from All columns", async () => {
        // given
        const user = userEvent.setup();
        renderControls({ ...shown, spotlight: makeSpotlight({ focused_column_id: STOP }) });

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Focus column" }), "All columns");

        // then
        expect(focus.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: null });
    });

    it("never offers a hidden column for focus", () => {
        // given
        const snapshot = {
            ...shown,
            columns: [
                makeColumn({ id: START, title: "Start", position: "a0" }),
                makeColumn({ id: STOP, title: "Stop", position: "a1", hidden: true }),
                makeColumn({ id: CONTINUE, title: "Continue", position: "a2" }),
            ],
        };

        // when
        renderControls(snapshot);

        // then
        const options = within(screen.getByRole("combobox", { name: "Focus column" })).getAllByRole("option");
        expect(options.map(option => option.textContent)).toEqual(["All columns", "Start", "Continue"]);
    });

    it("clears the highlight only while a card is highlighted", async () => {
        // given
        const user = userEvent.setup();
        const { unmount } = renderControls();
        const withoutHighlight = screen.queryByRole("button", { name: "Clear highlight" });
        unmount();
        renderControls({ ...shown, spotlight: makeSpotlight({ highlighted_card_id: "first" }) });

        // when
        await user.click(screen.getByRole("button", { name: "Clear highlight" }));

        // then
        expect(withoutHighlight).toBeNull();
        expect(highlight.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: null });
    });

    it("shows a refused spotlight change in the facilitator's words", () => {
        // given
        hooks.useFocusColumn.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "column hidden", code: "column_hidden" }),
            }),
        );

        // when
        renderControls();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "That column is hidden from participants, so nothing in it can change until it is shown again.",
        );
    });

    it("renders nothing for a participant or before the viewer has joined", () => {
        // when
        const { unmount } = renderControls({ ...shown, viewer: makeParticipantViewer() });
        const forParticipant = screen.queryByRole("region", { name: "Spotlight" });
        unmount();
        renderControls(shown, false);

        // then
        expect(forParticipant).toBeNull();
        expect(screen.queryByRole("region", { name: "Spotlight" })).not.toBeInTheDocument();
    });
});
