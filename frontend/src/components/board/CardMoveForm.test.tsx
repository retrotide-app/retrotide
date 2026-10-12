import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCardFocus } from "../../hooks/board/useCardFocus";
import { BOARD_ID, makeApiError, makeBoardSnapshot, makeCard, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { CardMoveForm } from "./CardMoveForm";

const { useMoveCard } = vi.hoisted(() => ({ useMoveCard: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useMoveCard }));

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const first = makeCard({ id: "k1", column_id: START, position: "a0", body: "First" });
const second = makeCard({ id: "k2", column_id: START, position: "a1", body: "Second" });
const snapshot = makeBoardSnapshot({ cards: [first, second] });

beforeEach(() => {
    vi.clearAllMocks();
    useMoveCard.mockReturnValue(makeMutationState());
});

describe("CardMoveForm", () => {
    it("moves the card to the first place on offer unless another is picked", async () => {
        // given
        const move = makeMutationState();
        useMoveCard.mockReturnValue(move);
        const cardFocus = createCardFocus();
        const user = userEvent.setup();
        renderWithProviders(<CardMoveForm snapshot={snapshot} card={first} cardFocus={cardFocus} />);

        // when
        await user.click(screen.getByRole("button", { name: "Move card" }));

        // then
        expect(useMoveCard).toHaveBeenCalledWith(BOARD_ID);
        expect(move.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k1", column_id: START, after_card_id: "k2" },
            expect.anything(),
        );
        expect(cardFocus.claim("k1")).toBe(false);
    });

    it("moves the card to the place picked from the keyboard-reachable list", async () => {
        // given
        const move = makeMutationState();
        useMoveCard.mockReturnValue(move);
        const user = userEvent.setup();
        renderWithProviders(<CardMoveForm snapshot={snapshot} card={second} cardFocus={createCardFocus()} />);

        // when
        await user.selectOptions(screen.getByLabelText("Move to"), "Top of Stop");
        await user.click(screen.getByRole("button", { name: "Move card" }));

        // then
        expect(move.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k2", column_id: STOP, after_card_id: null },
            expect.anything(),
        );
    });

    it("asks for focus to follow a card moved to another column", async () => {
        // given
        const cardFocus = createCardFocus();
        const user = userEvent.setup();
        renderWithProviders(<CardMoveForm snapshot={snapshot} card={second} cardFocus={cardFocus} />);

        // when
        await user.selectOptions(screen.getByLabelText("Move to"), "Top of Stop");
        await user.click(screen.getByRole("button", { name: "Move card" }));

        // then
        expect(cardFocus.claim("k2")).toBe(true);
    });

    it("withdraws the focus request when the move is refused", async () => {
        // given
        const move = makeMutationState();
        useMoveCard.mockReturnValue(move);
        const cardFocus = createCardFocus();
        const user = userEvent.setup();
        renderWithProviders(<CardMoveForm snapshot={snapshot} card={second} cardFocus={cardFocus} />);
        await user.selectOptions(screen.getByLabelText("Move to"), "Top of Stop");
        await user.click(screen.getByRole("button", { name: "Move card" }));

        // when
        const [, options] = move.mutate.mock.calls[0];
        options.onError();

        // then
        expect(cardFocus.claim("k2")).toBe(false);
    });

    it("shows a refused move in place", () => {
        // given
        useMoveCard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "moves are locked", code: "moves_locked" }),
            }),
        );

        // when
        renderWithProviders(<CardMoveForm snapshot={snapshot} card={first} cardFocus={createCardFocus()} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("moves are locked");
    });

    it("holds the button while the card moves", () => {
        // given
        useMoveCard.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<CardMoveForm snapshot={snapshot} card={first} cardFocus={createCardFocus()} />);

        // then
        expect(screen.getByRole("button", { name: "Move card" })).toBeDisabled();
    });
});
