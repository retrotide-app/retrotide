import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeApiError, makeBoardSnapshot, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { CsvImportResponse } from "../../types/api";
import { CsvImportSection } from "./CsvImportSection";

const { useImportCsv } = vi.hoisted(() => ({ useImportCsv: vi.fn() }));

vi.mock("../../hooks/mutations/boardManagement", () => ({ useImportCsv }));

function answeringWith(answer: CsvImportResponse) {
    const importer = makeMutationState();
    importer.mutate.mockImplementation(
        (_csv: string, options: { onSuccess: (response: CsvImportResponse) => void }) => {
            options.onSuccess(answer);
        },
    );
    useImportCsv.mockReturnValue(importer);

    return importer;
}

beforeEach(() => {
    vi.clearAllMocks();
    useImportCsv.mockReturnValue(makeMutationState());
});

describe("CsvImportSection", () => {
    it("explains the format beside a file picker and a text box", () => {
        // when
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);

        // then
        const section = screen.getByRole("region", { name: "Import cards" });
        expect(section).toHaveTextContent("The first row holds the column titles");
        expect(screen.getByLabelText("CSV file")).toHaveAttribute("type", "file");
        expect(screen.getByRole("textbox", { name: "CSV text" })).toHaveValue("");
        expect(useImportCsv).toHaveBeenCalledWith(BOARD_ID);
    });

    it("imports the pasted CSV and says how many cards and columns it added", async () => {
        // given
        const importer = answeringWith({ version: 9, columns_created: 1, cards_created: 3 });
        const user = userEvent.setup();
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);

        // when
        await user.type(screen.getByRole("textbox", { name: "CSV text" }), "Kudos{enter}Thanks Sam");
        await user.click(screen.getByRole("button", { name: "Import cards" }));

        // then
        expect(importer.mutate).toHaveBeenCalledWith("Kudos\nThanks Sam", expect.anything());
        expect(screen.getByRole("status")).toHaveTextContent("Added 3 cards and 1 new column.");
        expect(screen.getByRole("textbox", { name: "CSV text" })).toHaveValue("");
    });

    it("names only the cards when no column was added", async () => {
        // given
        answeringWith({ version: 9, columns_created: 0, cards_created: 1 });
        const user = userEvent.setup();
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);

        // when
        await user.type(screen.getByRole("textbox", { name: "CSV text" }), "Start{enter}More pairing");
        await user.click(screen.getByRole("button", { name: "Import cards" }));

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Added 1 card.");
    });

    it("reads a chosen file into the text box", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);
        const file = new File(["Went well\nShipped\n"], "retro.csv", { type: "text/csv" });

        // when
        await user.upload(screen.getByLabelText("CSV file"), file);

        // then
        expect(await screen.findByDisplayValue(/Went well/)).toHaveValue("Went well\nShipped\n");
    });

    it("asks for a CSV before sending nothing", async () => {
        // given
        const importer = makeMutationState();
        useImportCsv.mockReturnValue(importer);
        const user = userEvent.setup();
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);

        // when
        await user.click(screen.getByRole("button", { name: "Import cards" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Paste a CSV or choose a CSV file.");
        expect(importer.mutate).not.toHaveBeenCalled();
    });

    it("shows why the server refused the CSV", () => {
        // given
        useImportCsv.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 400,
                    message: "a CSV import can add at most 500 cards",
                    code: "csv_too_many_cards",
                }),
            }),
        );

        // when
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("a CSV import can add at most 500 cards");
    });

    it("holds the button while an import is on its way", () => {
        // given
        useImportCsv.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<CsvImportSection snapshot={makeBoardSnapshot()} />);

        // then
        expect(screen.getByRole("button", { name: "Import cards" })).toBeDisabled();
    });
});
