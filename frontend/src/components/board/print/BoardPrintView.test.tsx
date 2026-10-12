import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeBoardView, makeCard, makeColumn } from "../../../test-utils/fixtures";
import type { BoardSnapshot } from "../../../types/api";
import { BoardPrintView } from "./BoardPrintView";

const START = "00000000-0000-4000-8000-000000000e01";
const STOP = "00000000-0000-4000-8000-000000000e02";

function board(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ title: "Sprint 12", cards_hidden: false, stage: "discuss" }),
        columns: [
            makeColumn({ id: START, title: "Start", position: "a0" }),
            makeColumn({ id: STOP, title: "Stop", position: "a1" }),
        ],
        cards: [
            makeCard({
                id: "k1",
                column_id: START,
                position: "a0",
                body: "Pair on reviews",
                hidden: false,
                votes: { total: 3 },
                pinned: true,
                image_path: "/uploads/b1/m1.webp",
                reactions: [{ reaction: "party", count: 2 }],
            }),
            makeCard({
                id: "k2",
                column_id: START,
                position: "a1",
                parent_card_id: "k1",
                body: "Review in pairs",
                hidden: false,
                votes: { total: 1 },
            }),
            makeCard({
                id: "k3",
                column_id: STOP,
                position: "a0",
                body: "Mine, still hidden",
                hidden: true,
                votes: { total: null },
            }),
        ],
        hidden_counts: { [STOP]: 2 },
        ...overrides,
    });
}

describe("BoardPrintView", () => {
    it("lays every column side by side under the board's title", () => {
        // when
        render(<BoardPrintView snapshot={board()} />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
        expect(screen.getAllByRole("heading", { level: 2 }).map(heading => heading.textContent)).toEqual([
            "Start",
            "Stop",
        ]);
    });

    it("prints a card with its image, merged cards, pin, votes and reactions", () => {
        // when
        render(<BoardPrintView snapshot={board()} />);

        // then
        const start = screen.getByRole("region", { name: "Start" });
        expect(within(start).getByText("Pair on reviews")).toBeInTheDocument();
        expect(within(start).getByRole("img", { name: "Image on the card Pair on reviews" })).toHaveAttribute(
            "src",
            "/uploads/b1/m1.webp",
        );
        expect(within(start).getByRole("list", { name: "Merged cards" })).toHaveTextContent("Review in pairs");
        expect(within(start).getByText("Pinned")).toBeInTheDocument();
        expect(within(start).getByText("4 votes")).toBeInTheDocument();
        expect(within(start).getByRole("listitem", { name: "Party: 2" })).toHaveTextContent("🎉 2");
    });

    it("counts the cards still hidden from the viewer without showing them, and leaves a hidden total out", () => {
        // when
        render(<BoardPrintView snapshot={board()} />);

        // then
        const stop = screen.getByRole("region", { name: "Stop" });
        expect(within(stop).getByText("Mine, still hidden")).toBeInTheDocument();
        expect(within(stop).getByText("2 cards from others, hidden for now")).toBeInTheDocument();
        expect(within(stop).queryByText(/votes?$/)).not.toBeInTheDocument();
        expect(within(stop).queryByText("Pinned")).not.toBeInTheDocument();
    });
});
