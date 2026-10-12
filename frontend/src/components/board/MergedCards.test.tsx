import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardSnapshot, makeBoardView, makeCard, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { MergedCards } from "./MergedCards";

const { useUnmergeCard } = vi.hoisted(() => ({ useUnmergeCard: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useUnmergeCard }));

const top = makeCard({ id: "k1", body: "Deploys" });
const under = makeCard({
    id: "k2",
    parent_card_id: "k1",
    body: "Releases",
    author: { participant_id: "p-ali", display_name: "Ali" },
});

function renderMerged(snapshot: BoardSnapshot, writable = true) {
    return renderWithProviders(<MergedCards snapshot={snapshot} merged={[under]} writable={writable} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    useUnmergeCard.mockReturnValue(makeMutationState());
});

describe("MergedCards", () => {
    it("lists each merged card with its author and a way to take it out", () => {
        // when
        renderMerged(
            makeBoardSnapshot({ board: makeBoardView({ stage: "discuss", cards_hidden: false }), cards: [top, under] }),
        );

        // then
        const list = within(screen.getByRole("list", { name: "Merged cards" }));
        expect(list.getByText("Releases")).toBeInTheDocument();
        expect(list.getByText("Ali")).toBeInTheDocument();
        expect(list.getByRole("button", { name: 'Unmerge "Releases"' })).toBeInTheDocument();
    });

    it("offers no unmerge while cards are hidden or before the viewer has joined", () => {
        // when
        const { unmount } = renderMerged(makeBoardSnapshot({ cards: [top, under] }));
        const hidden = screen.queryByRole("button", { name: 'Unmerge "Releases"' });
        unmount();
        renderMerged(
            makeBoardSnapshot({ board: makeBoardView({ stage: "discuss", cards_hidden: false }), cards: [top, under] }),
            false,
        );

        // then
        expect(hidden).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: 'Unmerge "Releases"' })).not.toBeInTheDocument();
    });

    it.each(["write", "vote", "actions"] as const)("offers no unmerge in %s, only in Discuss", stage => {
        // when
        renderMerged(makeBoardSnapshot({ board: makeBoardView({ stage, cards_hidden: false }), cards: [top, under] }));

        // then
        expect(screen.getByRole("list", { name: "Merged cards" })).toHaveTextContent("Releases");
        expect(screen.queryByRole("button", { name: 'Unmerge "Releases"' })).not.toBeInTheDocument();
    });

    it("shows a refused unmerge", () => {
        // given
        useUnmergeCard.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "the card is not part of a merged stack" }),
            }),
        );

        // when
        renderMerged(
            makeBoardSnapshot({ board: makeBoardView({ stage: "discuss", cards_hidden: false }), cards: [top, under] }),
        );

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the card is not part of a merged stack");
    });
});
