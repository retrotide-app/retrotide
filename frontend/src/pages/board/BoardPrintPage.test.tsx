import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_PRINT_ROUTE } from "../../domain/board/print";
import { BOARD_ID, makeBoardSnapshot, makeBoardView, makeQueryState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { BoardPrintPage } from "./BoardPrintPage";

const { useBoard, printPage, exportNodeAsPng } = vi.hoisted(() => ({
    useBoard: vi.fn(),
    printPage: vi.fn(),
    exportNodeAsPng: vi.fn(),
}));

vi.mock("../../hooks/queries/boards", () => ({ useBoard }));
vi.mock("../../platform/print", () => ({ printPage }));
vi.mock("../../platform/imageExport", () => ({ exportNodeAsPng }));

const SNAPSHOT = makeBoardSnapshot({ board_id: BOARD_ID, board: makeBoardView({ title: "Sprint 12" }) });

function renderPage(search = "") {
    renderWithProviders(<BoardPrintPage />, { route: `/boards/${BOARD_ID}/print${search}`, path: BOARD_PRINT_ROUTE });
}

function loaded(snapshot: BoardSnapshot = SNAPSHOT) {
    useBoard.mockReturnValue(makeQueryState({ data: snapshot }));
}

beforeEach(() => {
    vi.resetAllMocks();
    exportNodeAsPng.mockResolvedValue(undefined);
    loaded();
});

describe("BoardPrintPage", () => {
    it("reads the board it was opened for and lays it out", () => {
        // when
        renderPage();

        // then
        expect(useBoard).toHaveBeenCalledWith(BOARD_ID);
        expect(screen.getByRole("heading", { level: 1, name: "Sprint 12" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to board" })).toHaveAttribute("href", `/boards/${BOARD_ID}`);
        expect(printPage).not.toHaveBeenCalled();
        expect(exportNodeAsPng).not.toHaveBeenCalled();
    });

    it("opens the print dialog once the board has drawn when asked to print", async () => {
        // when
        renderPage("?then=print");

        // then
        await waitFor(() => {
            expect(printPage).toHaveBeenCalledOnce();
        });
        expect(exportNodeAsPng).not.toHaveBeenCalled();
    });

    it("saves the drawn board as a PNG named like the exports when asked for one", async () => {
        // when
        renderPage("?then=png");

        // then
        await waitFor(() => {
            expect(exportNodeAsPng).toHaveBeenCalledOnce();
        });
        const [node, fileName] = exportNodeAsPng.mock.calls[0] ?? [];
        expect(node).toBe(screen.getByRole("article", { name: "Sprint 12" }));
        expect(fileName).toMatch(/^RetroTide - Sprint 12 - \d{4}-\d{2}-\d{2}\.png$/);
        expect(printPage).not.toHaveBeenCalled();
    });

    it("waits for the board before doing what it was asked", () => {
        // given
        useBoard.mockReturnValue(makeQueryState({ loading: true }));

        // when
        renderPage("?then=print");

        // then
        expect(screen.getByText("Loading board...")).toBeInTheDocument();
        expect(printPage).not.toHaveBeenCalled();
    });

    it("offers a retry when the board cannot be read", async () => {
        // given
        const user = userEvent.setup();
        const refetch = vi.fn().mockResolvedValue(undefined);
        useBoard.mockReturnValue(makeQueryState({ error: new Error("board not found"), refetch }));
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("board not found");
        expect(refetch).toHaveBeenCalledOnce();
    });

    it("prints and saves again from its own buttons", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Print or save as PDF" }));
        await user.click(screen.getByRole("button", { name: "Download PNG" }));

        // then
        expect(printPage).toHaveBeenCalledOnce();
        expect(exportNodeAsPng).toHaveBeenCalledOnce();
    });

    it("says so when the PNG cannot be drawn", async () => {
        // given
        exportNodeAsPng.mockRejectedValue(new Error("the canvas is tainted"));

        // when
        renderPage("?then=png");

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent("the canvas is tainted");
    });

    it("says so when the browser cannot print", async () => {
        // given
        printPage.mockImplementation(() => {
            throw new Error("This browser cannot print the page.");
        });

        // when
        renderPage("?then=print");

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent("This browser cannot print the page.");
    });
});
