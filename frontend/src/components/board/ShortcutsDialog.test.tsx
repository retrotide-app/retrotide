import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { ShortcutsDialog } from "./ShortcutsDialog";

describe("ShortcutsDialog", () => {
    it("lists every board shortcut with what it does", () => {
        // when
        renderWithProviders(<ShortcutsDialog open onClose={vi.fn()} />);

        // then
        const dialog = screen.getByRole("dialog", { name: "Keyboard shortcuts" });
        expect(within(dialog).getByText("?")).toBeInTheDocument();
        expect(within(dialog).getByText("Write a new card in the first column you can add to")).toBeInTheDocument();
        expect(within(dialog).getByText("Search the cards")).toBeInTheDocument();
        expect(within(dialog).getByText("Switch between the columns and list layouts")).toBeInTheDocument();
        expect(within(dialog).getByText("Close the open dialog")).toBeInTheDocument();
    });

    it("closes on Escape", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ShortcutsDialog open onClose={onClose} />);

        // when
        await user.keyboard("{Escape}");

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("renders nothing while closed", () => {
        // when
        renderWithProviders(<ShortcutsDialog open={false} onClose={vi.fn()} />);

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
});
