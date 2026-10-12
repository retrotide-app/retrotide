import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeBoardSnapshot, makeBoardView, makeCard, makeColumn } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { BoardExportMenu } from "./BoardExportMenu";

const { copyText } = vi.hoisted(() => ({ copyText: vi.fn() }));

vi.mock("../../platform/clipboard", () => ({ copyText }));

const COLUMN = "00000000-0000-4000-8000-000000000c01";

const SNAPSHOT = makeBoardSnapshot({
    board_id: BOARD_ID,
    board: makeBoardView({ title: "Sprint 12", cards_hidden: false, stage: "discuss" }),
    columns: [makeColumn({ id: COLUMN, title: "Start" })],
    cards: [makeCard({ column_id: COLUMN, body: "Pair on reviews", hidden: false, votes: { total: 2 } })],
});

beforeEach(() => {
    vi.resetAllMocks();
    copyText.mockResolvedValue(undefined);
});

describe("BoardExportMenu", () => {
    it("offers the board as a CSV, Markdown or JSON download", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<BoardExportMenu boardId={BOARD_ID} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        expect(screen.getByRole("menuitem", { name: "CSV" })).toHaveAttribute(
            "href",
            `/api/v1/boards/${BOARD_ID}/export?format=csv`,
        );
        expect(screen.getByRole("menuitem", { name: "Markdown" })).toHaveAttribute(
            "href",
            `/api/v1/boards/${BOARD_ID}/export?format=markdown`,
        );
        expect(screen.getByRole("menuitem", { name: "JSON" })).toHaveAttribute(
            "href",
            `/api/v1/boards/${BOARD_ID}/export?format=json`,
        );
        for (const name of ["CSV", "Markdown", "JSON"]) {
            expect(screen.getByRole("menuitem", { name })).toHaveAttribute("download");
        }
    });

    it("opens the print page in a new tab to print the board or save it as a PDF or a PNG", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<BoardExportMenu boardId={BOARD_ID} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        const print = screen.getByRole("menuitem", { name: "Print or save as PDF" });
        const png = screen.getByRole("menuitem", { name: "Download PNG" });
        expect(print).toHaveAttribute("href", `/boards/${BOARD_ID}/print?then=print`);
        expect(png).toHaveAttribute("href", `/boards/${BOARD_ID}/print?then=png`);
        expect(print).toHaveAttribute("target", "_blank");
        expect(png).toHaveAttribute("target", "_blank");
        expect(print).not.toHaveAttribute("download");
    });

    it("lists the formats, then the pictures, then copying", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<BoardExportMenu boardId={BOARD_ID} snapshot={SNAPSHOT} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        expect(screen.getAllByRole("menuitem").map(item => item.textContent)).toEqual([
            "CSV",
            "Markdown",
            "JSON",
            "Print or save as PDF",
            "Download PNG",
            "Copy board to clipboard",
        ]);
    });

    it("offers no copying without the board to copy", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<BoardExportMenu boardId={BOARD_ID} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));

        // then
        expect(screen.queryByRole("menuitem", { name: "Copy board to clipboard" })).not.toBeInTheDocument();
    });

    it("copies the board as text and says so", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<BoardExportMenu boardId={BOARD_ID} snapshot={SNAPSHOT} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));
        await user.click(screen.getByRole("menuitem", { name: "Copy board to clipboard" }));

        // then
        expect(copyText).toHaveBeenCalledExactlyOnceWith(
            ["# Sprint 12", "", "## Start", "- Pair on reviews (2 votes)"].join("\n"),
        );
        expect(await screen.findByRole("status")).toHaveTextContent("Board copied to the clipboard.");
    });

    it("says why the board could not be copied", async () => {
        // given
        const user = userEvent.setup();
        copyText.mockRejectedValue(new Error("This browser does not let the page copy text."));
        renderWithProviders(<BoardExportMenu boardId={BOARD_ID} snapshot={SNAPSHOT} />);

        // when
        await user.click(screen.getByRole("button", { name: "Export" }));
        await user.click(screen.getByRole("menuitem", { name: "Copy board to clipboard" }));

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent("This browser does not let the page copy text.");
    });
});
