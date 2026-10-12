import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DEFAULT_BOARD_QUERY } from "../../../domain/board/organisationBoards";
import { renderWithProviders } from "../../../test-utils/render";
import { BoardsToolbar } from "./BoardsToolbar";

const TEAM_OPTIONS = [
    { value: "all", label: "All teams" },
    { value: "organisation", label: "Organisation boards" },
    { value: "t1", label: "Crew" },
];

function renderToolbar(selecting?: { selecting: boolean; onSelectingChange: (selecting: boolean) => void }) {
    const onQueryChange = vi.fn();
    const onDisplayChange = vi.fn();

    renderWithProviders(
        <BoardsToolbar
            query={DEFAULT_BOARD_QUERY}
            onQueryChange={onQueryChange}
            display="grid"
            onDisplayChange={onDisplayChange}
            teamOptions={TEAM_OPTIONS}
            selecting={selecting?.selecting}
            onSelectingChange={selecting?.onSelectingChange}
        />,
    );

    return { onQueryChange, onDisplayChange };
}

describe("BoardsToolbar", () => {
    it("shows the filter, the team, the status, the pinned switch, the view and the order at their defaults", () => {
        // when
        renderToolbar();

        // then
        expect(screen.getByRole("searchbox", { name: "Filter your boards" })).toHaveValue("");
        expect(screen.getByRole("combobox", { name: "Team" })).toHaveValue("all");
        expect(screen.getByRole("combobox", { name: "Status" })).toHaveValue("active");
        expect(screen.getByRole("switch", { name: "Pinned only" })).not.toBeChecked();
        expect(screen.getByRole("button", { name: "Grid" })).toHaveAttribute("aria-pressed", "true");
        expect(screen.getByRole("combobox", { name: "Sort" })).toHaveValue("newest");
    });

    it("offers every status, recently deleted included", () => {
        // when
        renderToolbar();

        // then
        const options = screen.getAllByRole("option").map(option => option.textContent);
        expect(options).toEqual(
            expect.arrayContaining(["Active", "Ended", "Archived", "All boards", "Recently deleted"]),
        );
    });

    it("hands back the typed filter text", async () => {
        // given
        const user = userEvent.setup();
        const { onQueryChange } = renderToolbar();

        // when
        await user.type(screen.getByRole("searchbox", { name: "Filter your boards" }), "s");

        // then
        expect(onQueryChange).toHaveBeenCalledExactlyOnceWith({ ...DEFAULT_BOARD_QUERY, text: "s" });
    });

    it("hands back the picked team, status, pin switch and order", async () => {
        // given
        const user = userEvent.setup();
        const { onQueryChange } = renderToolbar();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Team" }), "Crew");
        await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), "Archived");
        await user.click(screen.getByRole("switch", { name: "Pinned only" }));
        await user.selectOptions(screen.getByRole("combobox", { name: "Sort" }), "Oldest first");

        // then
        expect(onQueryChange).toHaveBeenNthCalledWith(1, { ...DEFAULT_BOARD_QUERY, team: "t1" });
        expect(onQueryChange).toHaveBeenNthCalledWith(2, { ...DEFAULT_BOARD_QUERY, status: "archived" });
        expect(onQueryChange).toHaveBeenNthCalledWith(3, { ...DEFAULT_BOARD_QUERY, pinnedOnly: true });
        expect(onQueryChange).toHaveBeenNthCalledWith(4, { ...DEFAULT_BOARD_QUERY, sort: "oldest" });
    });

    it("hands back the picked view", async () => {
        // given
        const user = userEvent.setup();
        const { onDisplayChange } = renderToolbar();

        // when
        await user.click(screen.getByRole("button", { name: "List" }));

        // then
        expect(onDisplayChange).toHaveBeenCalledExactlyOnceWith("list");
    });

    it("offers no selection mode unless it can be used", () => {
        // when
        renderToolbar();

        // then
        expect(screen.queryByRole("button", { name: "Select" })).not.toBeInTheDocument();
    });

    it("enters selection mode from the Select toggle", async () => {
        // given
        const onSelectingChange = vi.fn();
        const user = userEvent.setup();
        renderToolbar({ selecting: false, onSelectingChange });

        // when
        await user.click(screen.getByRole("button", { name: "Select" }));

        // then
        expect(screen.getByRole("button", { name: "Select" })).toHaveAttribute("aria-pressed", "false");
        expect(onSelectingChange).toHaveBeenCalledExactlyOnceWith(true);
    });

    it("leaves selection mode from the pressed Select toggle", async () => {
        // given
        const onSelectingChange = vi.fn();
        const user = userEvent.setup();
        renderToolbar({ selecting: true, onSelectingChange });

        // when
        await user.click(screen.getByRole("button", { name: "Select" }));

        // then
        expect(screen.getByRole("button", { name: "Select" })).toHaveAttribute("aria-pressed", "true");
        expect(onSelectingChange).toHaveBeenCalledExactlyOnceWith(false);
    });
});
