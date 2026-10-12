import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeBoardListItem } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { BoardCard } from "./BoardCard";

describe("BoardCard", () => {
    it("names the board in a heading whose link opens it", () => {
        // given
        const board = makeBoardListItem({ id: "board-sprint", title: "Sprint 12" });

        // when
        renderWithProviders(<BoardCard board={board} href="/o/harbour/boards/board-sprint" />);

        // then
        const heading = screen.getByRole("heading", { name: "Sprint 12" });
        expect(within(heading).getByRole("link", { name: "Sprint 12" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/board-sprint",
        );
    });

    it("names who created the board", () => {
        // given
        const board = makeBoardListItem({ title: "Sprint 12", creator_name: "Hana" });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(screen.getByText(/Created by/)).toHaveTextContent("Created by Hana");
    });

    it("leaves the creator out when the account that made the board is gone", () => {
        // given
        const board = makeBoardListItem({ title: "Sprint 12", creator_name: undefined });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(screen.queryByText(/Created by/)).not.toBeInTheDocument();
    });

    it("shows when the board was created", () => {
        // given
        const board = makeBoardListItem({ title: "Sprint 12", created_at: "2026-10-01T09:00:00Z" });

        // when
        const { container } = renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(container.querySelector("time")).toHaveAttribute("dateTime", "2026-10-01T09:00:00Z");
    });

    it("leaves the date out for an anonymous board that has none", () => {
        // given
        const board = makeBoardListItem({ title: "Quiet board", created_at: undefined });

        // when
        const { container } = renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(container.querySelector("time")).toBeNull();
    });

    it("tags an archived board", () => {
        // given
        const board = makeBoardListItem({ title: "Old board", archived_at: "2026-09-01T09:00:00Z" });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(screen.getByText("Archived")).toBeInTheDocument();
    });

    it("has no archived tag on a live board", () => {
        // given
        const board = makeBoardListItem({ title: "Live board", archived_at: undefined });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(screen.queryByText("Archived")).not.toBeInTheDocument();
    });

    it("offers no pin and no selection unless asked to", () => {
        // given
        const board = makeBoardListItem({ title: "Sprint 12" });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" />);

        // then
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
        expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    });

    it("pins an unpinned board", async () => {
        // given
        const board = makeBoardListItem({ id: "b1", title: "Sprint 12" });
        const onPin = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<BoardCard board={board} href="/boards/one" onPin={onPin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Pin board" }));

        // then
        expect(screen.getByRole("button", { name: "Pin board" })).toHaveAttribute("aria-pressed", "false");
        expect(onPin).toHaveBeenCalledExactlyOnceWith(board);
    });

    it("unpins a pinned board", async () => {
        // given
        const board = { ...makeBoardListItem({ id: "b1", title: "Sprint 12" }), pinned: true as const };
        const onPin = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<BoardCard board={board} href="/boards/one" onPin={onPin} />);

        // when
        await user.click(screen.getByRole("button", { name: "Unpin board" }));

        // then
        expect(screen.getByRole("button", { name: "Unpin board" })).toHaveAttribute("aria-pressed", "true");
        expect(onPin).toHaveBeenCalledExactlyOnceWith(board);
    });

    it("holds the pin while a pin is on its way", () => {
        // given
        const board = makeBoardListItem({ title: "Sprint 12" });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" onPin={vi.fn()} pinBusy />);

        // then
        expect(screen.getByRole("button", { name: "Pin board" })).toBeDisabled();
    });

    it("selects the board through its checkbox", async () => {
        // given
        const board = makeBoardListItem({ id: "b1", title: "Sprint 12" });
        const onSelect = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<BoardCard board={board} href="/boards/one" selected={false} onSelect={onSelect} />);

        // when
        await user.click(screen.getByRole("checkbox", { name: "Select Sprint 12" }));

        // then
        expect(screen.getByRole("checkbox", { name: "Select Sprint 12" })).not.toBeChecked();
        expect(onSelect).toHaveBeenCalledExactlyOnceWith(board);
    });

    it("shows a selected board as checked", () => {
        // given
        const board = makeBoardListItem({ title: "Sprint 12" });

        // when
        renderWithProviders(<BoardCard board={board} href="/boards/one" selected onSelect={vi.fn()} />);

        // then
        expect(screen.getByRole("checkbox", { name: "Select Sprint 12" })).toBeChecked();
    });
});
