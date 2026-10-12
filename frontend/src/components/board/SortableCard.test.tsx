import { DragDropProvider } from "@dnd-kit/react";
import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCardFocus } from "../../hooks/board/useCardFocus";
import { makeBoardSnapshot, makeCard, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { SortableCard } from "./SortableCard";

const mutations = vi.hoisted(() => ({
    useEditCard: vi.fn(),
    useRecolourCard: vi.fn(),
    useDeleteCard: vi.fn(),
    useMoveCard: vi.fn(),
    useCastVote: vi.fn(),
    useRemoveVote: vi.fn(),
    useMergeCard: vi.fn(),
    useUnmergeCard: vi.fn(),
    useCreateComment: vi.fn(),
    useEditComment: vi.fn(),
    useDeleteComment: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => mutations);

const card = makeCard({ body: "Pair on reviews" });
const snapshot = makeBoardSnapshot({ cards: [card] });

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
});

describe("SortableCard", () => {
    it("lists the card, draggable as a whole, when the card may move", () => {
        // when
        renderWithProviders(
            <DragDropProvider>
                <ul>
                    <SortableCard
                        snapshot={snapshot}
                        stack={{ card, merged: [] }}
                        index={0}
                        writable
                        movable
                        cardFocus={createCardFocus()}
                    />
                </ul>
            </DragDropProvider>,
        );

        // then
        expect(screen.getByRole("listitem")).toContainElement(screen.getByRole("article", { name: "Pair on reviews" }));
        expect(screen.queryByRole("button", { name: "Drag card" })).not.toBeInTheDocument();
    });

    it("lists the card when the viewer may not move it", () => {
        // when
        renderWithProviders(
            <DragDropProvider>
                <ul>
                    <SortableCard
                        snapshot={snapshot}
                        stack={{ card, merged: [] }}
                        index={0}
                        writable={false}
                        movable={false}
                        cardFocus={createCardFocus()}
                    />
                </ul>
            </DragDropProvider>,
        );

        // then
        expect(screen.getByRole("article", { name: "Pair on reviews" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Drag card" })).not.toBeInTheDocument();
    });

    it("shows the cards merged into the stack inside its top card", () => {
        // given
        const under = makeCard({ id: "k2", body: "Review faster", parent_card_id: card.id });

        // when
        renderWithProviders(
            <DragDropProvider>
                <ul>
                    <SortableCard
                        snapshot={makeBoardSnapshot({ cards: [card, under] })}
                        stack={{ card, merged: [under] }}
                        index={0}
                        writable
                        movable
                        mergeable
                        cardFocus={createCardFocus()}
                    />
                </ul>
            </DragDropProvider>,
        );

        // then
        const article = screen.getByRole("article", { name: "Pair on reviews" });
        expect(within(article).getByRole("list", { name: "Merged cards" })).toHaveTextContent("Review faster");
    });
});
