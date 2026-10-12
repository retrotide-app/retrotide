import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardSnapshot, makeCard, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, CardView } from "../../types/api";
import { CardMergeForm } from "./CardMergeForm";

const { useMergeCard } = vi.hoisted(() => ({ useMergeCard: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useMergeCard }));

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const dragged = makeCard({ id: "k1", column_id: START, position: "a0", body: "Deploys" });
const standups = makeCard({ id: "k2", column_id: STOP, position: "a0", body: "Standups" });
const meetings = makeCard({ id: "k3", column_id: STOP, position: "a1", body: "Meetings" });

const onMerged = vi.fn();

let merge = makeMutationState();

function renderForm(
    card: CardView = dragged,
    snapshot: BoardSnapshot = makeBoardSnapshot({ cards: [dragged, standups, meetings] }),
) {
    return renderWithProviders(<CardMergeForm snapshot={snapshot} card={card} onMerged={onMerged} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    merge = makeMutationState();
    useMergeCard.mockReturnValue(merge);
});

describe("CardMergeForm", () => {
    it("lists every other card by column and merges into the one picked", async () => {
        // given
        const user = userEvent.setup();
        renderForm();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Merge into" }), "Stop: Meetings");
        await user.click(screen.getByRole("button", { name: "Merge cards" }));

        // then
        const options = within(screen.getByRole("combobox", { name: "Merge into" })).getAllByRole("option");
        expect(options.map(option => option.textContent)).toEqual(["Stop: Standups", "Stop: Meetings"]);
        expect(merge.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k1", targetCardId: "k3" },
            { onSuccess: onMerged },
        );
    });

    it("shows a refused merge in the form", () => {
        // given
        useMergeCard.mockReturnValue(
            makeMutationState({
                error: makeApiError({
                    status: 409,
                    message: "merging cards is switched off",
                    code: "merging_disabled",
                }),
            }),
        );

        // when
        renderForm();

        // then
        expect(screen.getByRole("form", { name: "Merge card" })).toHaveTextContent("merging cards is switched off");
    });

    it("says so when there is no other card to merge into", () => {
        // when
        renderForm(dragged, makeBoardSnapshot({ cards: [dragged] }));

        // then
        expect(screen.getByText("There is no other card to merge this one into.")).toBeInTheDocument();
    });
});
