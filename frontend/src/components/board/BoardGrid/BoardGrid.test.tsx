import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { BoardGrid } from "./BoardGrid";

const BOARDS = [
    { id: "board-one", title: "Sprint 1 retro" },
    { id: "board-two", title: "Sprint 2 retro" },
];

describe("BoardGrid", () => {
    it("lists each board as a link to its page", () => {
        // when
        renderWithProviders(<BoardGrid boards={BOARDS} boardHref={id => `/boards/${id}`} label="Boards" />);

        // then
        const list = screen.getByRole("list", { name: "Boards" });
        expect(within(list).getAllByRole("listitem")).toHaveLength(2);
        expect(within(list).getByRole("link", { name: "Sprint 1 retro" })).toHaveAttribute("href", "/boards/board-one");
        expect(within(list).getByRole("link", { name: "Sprint 2 retro" })).toHaveAttribute("href", "/boards/board-two");
    });

    it("puts the Add board tile first when given somewhere to add a board", () => {
        // when
        renderWithProviders(
            <BoardGrid boards={BOARDS} boardHref={id => `/boards/${id}`} addHref="/boards/new" label="Boards" />,
        );

        // then
        const links = within(screen.getByRole("list", { name: "Boards" })).getAllByRole("link");
        expect(links[0]).toHaveAccessibleName("Add board");
        expect(links[0]).toHaveAttribute("href", "/boards/new");
    });

    it("leaves out the Add board tile when there is nowhere to add one", () => {
        // when
        renderWithProviders(<BoardGrid boards={BOARDS} boardHref={id => `/boards/${id}`} label="Boards" />);

        // then
        expect(screen.queryByRole("link", { name: "Add board" })).not.toBeInTheDocument();
    });
});
