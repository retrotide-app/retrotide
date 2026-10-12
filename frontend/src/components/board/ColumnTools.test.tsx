import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    BOARD_ID,
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeMutationState,
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { ColumnView } from "../../types/api";
import { ColumnTools } from "./ColumnTools";

const { useUpdateColumn, useMoveColumn, useClearColumn, useDeleteColumn } = vi.hoisted(() => ({
    useUpdateColumn: vi.fn(),
    useMoveColumn: vi.fn(),
    useClearColumn: vi.fn(),
    useDeleteColumn: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => ({ useUpdateColumn, useMoveColumn, useClearColumn, useDeleteColumn }));

const snapshot = makeBoardSnapshot();
const [start, stop, keep] = snapshot.columns;

function renderTools(column: ColumnView) {
    return renderWithProviders(<ColumnTools snapshot={snapshot} column={column} />);
}

async function openMenu(column: ColumnView) {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: `Options for ${column.title}` }));

    return user;
}

async function itemsOf(column: ColumnView): Promise<string[]> {
    await openMenu(column);

    return within(screen.getByRole("menu"))
        .getAllByRole("menuitem")
        .map(item => item.textContent ?? "");
}

async function choose(column: ColumnView, item: string) {
    const user = await openMenu(column);
    await user.click(screen.getByRole("menuitem", { name: item }));

    return user;
}

beforeEach(() => {
    vi.clearAllMocks();
    useUpdateColumn.mockReturnValue(makeMutationState());
    useMoveColumn.mockReturnValue(makeMutationState());
    useClearColumn.mockReturnValue(makeMutationState());
    useDeleteColumn.mockReturnValue(makeMutationState());
});

describe("ColumnTools", () => {
    it("puts every column action in one menu named after the column", async () => {
        // when
        renderTools(stop);
        const items = await itemsOf(stop);

        // then
        expect(items).toEqual(["Edit column", "Move left", "Move right", "Clear cards", "Delete column"]);
    });

    it("leaves clearing and deleting to facilitators when anyone can edit", async () => {
        // given a participant shaping columns because anyone can edit
        const shaping = makeBoardSnapshot({
            viewer: makeParticipantViewer(),
            settings: makeBoardSettings({ anyone_can_edit: true }),
        });
        renderWithProviders(<ColumnTools snapshot={shaping} column={stop} />);

        // when
        const items = await itemsOf(stop);

        // then
        expect(items).toEqual(["Edit column", "Move left", "Move right"]);
    });

    it("offers no move left for the first column and no move right for the last", async () => {
        // when
        const { unmount } = renderTools(start);
        const first = await itemsOf(start);
        unmount();
        renderTools(keep);
        const last = await itemsOf(keep);

        // then
        expect(first).not.toContain("Move left");
        expect(first).toContain("Move right");
        expect(last).toContain("Move left");
        expect(last).not.toContain("Move right");
    });

    it("renames, describes and recolours the column in a dialog", async () => {
        // given
        const update = makeMutationState();
        update.mutate.mockImplementation((_variables, options) => options.onSuccess());
        useUpdateColumn.mockReturnValue(update);
        renderTools(stop);
        const user = await choose(stop, "Edit column");

        // when
        const title = screen.getByLabelText("Column title");
        await user.clear(title);
        await user.type(title, "Stop doing");
        await user.clear(screen.getByLabelText("Description"));
        await user.selectOptions(screen.getByLabelText("Column colour"), "Pink");
        await user.click(screen.getByRole("button", { name: "Save column" }));

        // then
        expect(useUpdateColumn).toHaveBeenCalledWith(BOARD_ID);
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(
            { columnId: stop.id, title: "Stop doing", description: "", colour: "pink" },
            expect.anything(),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("moves the column one place left or right", async () => {
        // given
        const move = makeMutationState();
        useMoveColumn.mockReturnValue(move);
        renderTools(stop);

        // when
        await choose(stop, "Move left");
        await choose(stop, "Move right");

        // then
        expect(move.mutate).toHaveBeenNthCalledWith(1, { columnId: stop.id, after_column_id: null });
        expect(move.mutate).toHaveBeenNthCalledWith(2, { columnId: stop.id, after_column_id: keep.id });
    });

    it("says why a move was refused", () => {
        // given
        useMoveColumn.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 404, message: "column not found" }) }),
        );

        // when
        renderTools(stop);

        // then
        expect(screen.getByText("column not found")).toBeInTheDocument();
    });

    it("clears the cards once the facilitator confirms", async () => {
        // given
        const clear = makeMutationState();
        useClearColumn.mockReturnValue(clear);
        renderTools(stop);
        const user = await choose(stop, "Clear cards");

        // when
        const dialog = screen.getByRole("dialog", { name: "Clear the cards in Stop?" });
        await user.click(within(dialog).getByRole("button", { name: "Clear cards" }));

        // then
        expect(dialog).toHaveTextContent("including cards that are still hidden");
        expect(clear.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: stop.id }, expect.anything());
    });

    it("deletes the column once the facilitator confirms, and not on cancel", async () => {
        // given
        const removal = makeMutationState();
        useDeleteColumn.mockReturnValue(removal);
        renderTools(stop);

        // when
        let user = await choose(stop, "Delete column");
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete Stop?" })).getByRole("button", { name: "Cancel" }),
        );
        user = await choose(stop, "Delete column");
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete Stop?" })).getByRole("button", {
                name: "Delete column",
            }),
        );

        // then
        expect(removal.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: stop.id }, expect.anything());
    });

    it("shows a refused delete in the dialog", async () => {
        // given
        useDeleteColumn.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "the board is read only", code: "read_only" }),
            }),
        );
        renderTools(stop);

        // when
        await choose(stop, "Delete column");

        // then
        expect(screen.getByRole("dialog", { name: "Delete Stop?" })).toHaveTextContent("the board is read only");
    });
});
