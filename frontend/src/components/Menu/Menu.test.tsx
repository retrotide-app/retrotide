import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, type Mock, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Menu, type MenuItem } from "./Menu";

function makeItems(): { items: MenuItem[]; rename: Mock; remove: Mock } {
    const rename = vi.fn();
    const remove = vi.fn();

    return {
        items: [
            { label: "Rename", onSelect: rename },
            { label: "Archive", onSelect: vi.fn() },
            { label: "Delete", onSelect: remove, tone: "danger" },
        ],
        rename,
        remove,
    };
}

describe("Menu", () => {
    it("starts closed, with a trigger that says it opens a menu", () => {
        // given
        const { items } = makeItems();

        // when
        renderWithProviders(<Menu label="Team settings" items={items} />);

        // then
        const trigger = screen.getByRole("button", { name: "Team settings" });
        expect(trigger).toHaveAttribute("aria-haspopup", "menu");
        expect(trigger).toHaveAttribute("aria-expanded", "false");
        expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });

    it("opens on click and focuses the first item", async () => {
        // given
        const user = userEvent.setup();
        const { items } = makeItems();
        renderWithProviders(<Menu label="Team settings" items={items} />);

        // when
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // then
        expect(screen.getByRole("menu", { name: "Team settings" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Team settings" })).toHaveAttribute("aria-expanded", "true");
        expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus();
    });

    it("opens outside its container so a scrolling parent cannot clip it", async () => {
        // given
        const user = userEvent.setup();
        const { items } = makeItems();
        renderWithProviders(
            <section aria-label="Scroller" style={{ overflowX: "auto" }}>
                <Menu label="Team settings" items={items} />
            </section>,
        );

        // when
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // then
        const menu = screen.getByRole("menu", { name: "Team settings" });
        expect(screen.getByRole("region", { name: "Scroller" })).not.toContainElement(menu);
        expect(menu.parentElement).toBe(document.body);
    });

    it("runs the chosen item, closes, and hands focus back to the trigger", async () => {
        // given
        const user = userEvent.setup();
        const { items, remove } = makeItems();
        renderWithProviders(<Menu label="Team settings" items={items} />);
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // when
        await user.click(screen.getByRole("menuitem", { name: "Delete" }));

        // then
        expect(remove).toHaveBeenCalledTimes(1);
        expect(screen.queryByRole("menu")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Team settings" })).toHaveFocus();
    });

    it("moves between items with the arrow keys, wrapping at either end", async () => {
        // given
        const user = userEvent.setup();
        const { items } = makeItems();
        renderWithProviders(<Menu label="Team settings" items={items} />);
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // when
        await user.keyboard("{ArrowUp}");

        // then
        expect(screen.getByRole("menuitem", { name: "Delete" })).toHaveFocus();

        // when
        await user.keyboard("{ArrowDown}");

        // then
        expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus();

        // when
        await user.keyboard("{End}");

        // then
        expect(screen.getByRole("menuitem", { name: "Delete" })).toHaveFocus();

        // when
        await user.keyboard("{Home}");

        // then
        expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus();
    });

    it("chooses the focused item with Enter", async () => {
        // given
        const user = userEvent.setup();
        const { items, rename } = makeItems();
        renderWithProviders(<Menu label="Team settings" items={items} />);
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // when
        await user.keyboard("{Enter}");

        // then
        expect(rename).toHaveBeenCalledTimes(1);
    });

    it("closes on Escape without choosing anything", async () => {
        // given
        const user = userEvent.setup();
        const { items, rename, remove } = makeItems();
        renderWithProviders(<Menu label="Team settings" items={items} />);
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // when
        await user.keyboard("{Escape}");

        // then
        expect(screen.queryByRole("menu")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Team settings" })).toHaveFocus();
        expect(rename).not.toHaveBeenCalled();
        expect(remove).not.toHaveBeenCalled();
    });

    it("closes when the pointer goes down outside it", async () => {
        // given
        const user = userEvent.setup();
        const { items } = makeItems();
        renderWithProviders(
            <>
                <p>Elsewhere</p>
                <Menu label="Team settings" items={items} />
            </>,
        );
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // when
        await user.click(screen.getByText("Elsewhere"));

        // then
        expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });

    it("opens from the keyboard with the down arrow", async () => {
        // given
        const user = userEvent.setup();
        const { items } = makeItems();
        renderWithProviders(<Menu label="Team settings" items={items} />);
        screen.getByRole("button", { name: "Team settings" }).focus();

        // when
        await user.keyboard("{ArrowDown}");

        // then
        expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus();
    });

    it("names an icon-only trigger by its label", () => {
        // given
        const { items } = makeItems();

        // when
        renderWithProviders(<Menu label="Actions for Robin" items={items} iconOnly />);

        // then
        expect(screen.getByRole("button", { name: "Actions for Robin" })).toBeInTheDocument();
    });

    it("draws a link item as a download link that the arrow keys still reach", async () => {
        // given
        const user = userEvent.setup();
        const items: MenuItem[] = [
            { label: "CSV", href: "/files/board.csv", download: true },
            { label: "Open", href: "/files/board" },
        ];
        renderWithProviders(<Menu label="Export" items={items} />);
        await user.click(screen.getByRole("button", { name: "Export" }));

        // when
        await user.keyboard("{ArrowDown}");

        // then
        const csv = screen.getByRole("menuitem", { name: "CSV" });
        const open = screen.getByRole("menuitem", { name: "Open" });
        expect(csv).toHaveAttribute("href", "/files/board.csv");
        expect(csv).toHaveAttribute("download", "");
        expect(open).toHaveAttribute("href", "/files/board");
        expect(open).not.toHaveAttribute("download");
        expect(open).toHaveFocus();
    });

    it("opens a new-tab link in a tab of its own without handing it this page", async () => {
        // given
        const user = userEvent.setup();
        const items: MenuItem[] = [
            { label: "Print", href: "/boards/b1/print?then=print", newTab: true },
            { label: "Open", href: "/files/board" },
        ];
        renderWithProviders(<Menu label="Export" items={items} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        const print = screen.getByRole("menuitem", { name: "Print" });
        const open = screen.getByRole("menuitem", { name: "Open" });
        expect(print).toHaveAttribute("target", "_blank");
        expect(print).toHaveAttribute("rel", "noopener");
        expect(open).not.toHaveAttribute("target");
        expect(open).not.toHaveAttribute("rel");
    });

    it("renders nothing when it has no items", () => {
        // when
        renderWithProviders(<Menu label="Team settings" items={[]} />);

        // then
        expect(screen.queryByRole("button", { name: "Team settings" })).not.toBeInTheDocument();
    });
});
