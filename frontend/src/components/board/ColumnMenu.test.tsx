import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { boardActions } from "../../domain/board/capabilities";
import {
    BOARD_ID,
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeColumn,
    makeMutationState,
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, ColumnView } from "../../types/api";
import { ColumnMenu } from "./ColumnMenu";

const boards = vi.hoisted(() => ({ useUpdateColumn: vi.fn(), useMoveColumn: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => boards);

const facilitation = vi.hoisted(() => ({
    useRevealColumn: vi.fn(),
    useHideColumn: vi.fn(),
    useShowColumn: vi.fn(),
    useResetColumnVotes: vi.fn(),
    useSetColumnVoteLimit: vi.fn(),
}));

vi.mock("../../hooks/mutations/facilitation", () => facilitation);

const clipboard = vi.hoisted(() => ({ copyText: vi.fn() }));

vi.mock("../../platform/clipboard", () => clipboard);

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const CONTINUE = "00000000-0000-4000-8000-000000000c03";

function columnOf(snapshot: BoardSnapshot, id: string): ColumnView {
    const column = snapshot.columns.find(held => held.id === id);

    if (column === undefined) {
        throw new Error(`no column ${id}`);
    }

    return column;
}

function renderMenu(snapshot: BoardSnapshot, columnId: string = STOP) {
    return renderWithProviders(
        <ColumnMenu snapshot={snapshot} column={columnOf(snapshot, columnId)} actions={boardActions(snapshot)} />,
    );
}

async function openMenu(title = "Stop") {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: `Options for ${title}` }));

    return user;
}

async function itemsOf(title = "Stop"): Promise<string[]> {
    await openMenu(title);

    return within(screen.getByRole("menu"))
        .getAllByRole("menuitem")
        .map(item => item.textContent ?? "");
}

async function choose(item: string, title = "Stop") {
    const user = await openMenu(title);
    await user.click(screen.getByRole("menuitem", { name: item }));

    return user;
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of [...Object.values(boards), ...Object.values(facilitation)]) {
        hook.mockReturnValue(makeMutationState());
    }
    clipboard.copyText.mockResolvedValue(undefined);
});

describe("ColumnMenu", () => {
    it("gives a facilitator every column action while cards are hidden in the Write stage", async () => {
        // when
        renderMenu(makeBoardSnapshot());
        const items = await itemsOf();

        // then
        expect(items).toEqual([
            "Edit column",
            "Move left",
            "Move right",
            "Reveal cards",
            "Hide from participants",
            "Vote limit…",
            "Copy column",
            "Reset votes",
        ]);
    });

    it("offers no reveal once the column's cards are shown or the board has left Write", async () => {
        // when
        const revealed = makeBoardSnapshot({
            columns: [makeColumn(), makeColumn({ id: STOP, title: "Stop", position: "a1", cards_revealed: true })],
        });
        const { unmount } = renderMenu(revealed);
        const afterReveal = await itemsOf();
        unmount();
        renderMenu(makeBoardSnapshot({ board: makeBoardView({ stage: "vote", cards_hidden: false }) }));
        const voting = await itemsOf();

        // then
        expect(afterReveal).not.toContain("Reveal cards");
        expect(voting).not.toContain("Reveal cards");
    });

    it("offers to show a hidden column to everyone, and nothing that would change its cards", async () => {
        // given
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn(), makeColumn({ id: STOP, title: "Stop", position: "a1", hidden: true })],
        });

        // when
        renderMenu(snapshot);
        const items = await itemsOf();

        // then
        expect(items).toContain("Show to everyone");
        expect(items).toContain("Vote limit…");
        expect(items).not.toContain("Hide from participants");
        expect(items).not.toContain("Reveal cards");
        expect(items).not.toContain("Reset votes");
    });

    it("lets a seated participant shape columns when anyone can edit, and nothing more", async () => {
        // given
        const snapshot = makeBoardSnapshot({
            viewer: makeParticipantViewer(),
            settings: makeBoardSettings({ anyone_can_edit: true }),
        });

        // when
        renderMenu(snapshot);
        const items = await itemsOf();

        // then
        expect(items).toEqual(["Edit column", "Move left", "Move right", "Copy column"]);
    });

    it("offers a participant only copying when only facilitators shape columns", async () => {
        // when
        renderMenu(makeBoardSnapshot({ viewer: makeParticipantViewer() }));
        const items = await itemsOf();

        // then
        expect(items).toEqual(["Copy column"]);
    });

    it("copies the column's title and cards and says so", async () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ stage: "discuss", cards_hidden: false }),
            cards: [makeCard({ id: "k1", column_id: STOP, body: "Long standups", hidden: false })],
        });
        renderMenu(snapshot);

        // when
        await choose("Copy column");

        // then
        expect(clipboard.copyText).toHaveBeenCalledExactlyOnceWith("## Stop\n- Long standups");
        expect(await screen.findByText("Column copied.")).toBeInTheDocument();
    });

    it("says why copying the column failed", async () => {
        // given
        clipboard.copyText.mockRejectedValue(new Error("This browser does not let the page copy text."));
        renderMenu(makeBoardSnapshot());

        // when
        await choose("Copy column");

        // then
        expect(await screen.findByText("This browser does not let the page copy text.")).toBeInTheDocument();
    });

    it("reveals the column's cards and hides the column", async () => {
        // given
        const reveal = makeMutationState();
        const hide = makeMutationState();
        facilitation.useRevealColumn.mockReturnValue(reveal);
        facilitation.useHideColumn.mockReturnValue(hide);
        renderMenu(makeBoardSnapshot());

        // when
        await choose("Reveal cards");
        await choose("Hide from participants");

        // then
        expect(facilitation.useRevealColumn).toHaveBeenCalledWith(BOARD_ID);
        expect(reveal.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: STOP });
        expect(hide.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: STOP });
    });

    it("shows a hidden column to everyone from the menu", async () => {
        // given
        const show = makeMutationState();
        facilitation.useShowColumn.mockReturnValue(show);
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn(), makeColumn({ id: STOP, title: "Stop", position: "a1", hidden: true })],
        });
        renderMenu(snapshot);

        // when
        await choose("Show to everyone");

        // then
        expect(show.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: STOP });
    });

    it("moves the column one place left or right", async () => {
        // given
        const move = makeMutationState();
        boards.useMoveColumn.mockReturnValue(move);
        renderMenu(makeBoardSnapshot());

        // when
        await choose("Move left");
        await choose("Move right");

        // then
        expect(move.mutate).toHaveBeenNthCalledWith(1, { columnId: STOP, after_column_id: null });
        expect(move.mutate).toHaveBeenNthCalledWith(2, { columnId: STOP, after_column_id: CONTINUE });
    });

    it("edits the column in a dialog", async () => {
        // given
        const update = makeMutationState();
        update.mutate.mockImplementation((_variables, options) => options.onSuccess());
        boards.useUpdateColumn.mockReturnValue(update);
        renderMenu(makeBoardSnapshot());
        const user = await choose("Edit column");

        // when
        const title = screen.getByLabelText("Column title");
        await user.clear(title);
        await user.type(title, "Stop doing");
        await user.click(screen.getByRole("button", { name: "Save column" }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(
            { columnId: STOP, title: "Stop doing", description: "What should we stop doing?", colour: "red" },
            expect.anything(),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("resets the column's votes only once the facilitator confirms", async () => {
        // given
        const reset = makeMutationState();
        facilitation.useResetColumnVotes.mockReturnValue(reset);
        renderMenu(makeBoardSnapshot());

        // when
        let user = await choose("Reset votes");
        await user.click(
            within(screen.getByRole("dialog", { name: "Reset the votes in Stop?" })).getByRole("button", {
                name: "Cancel",
            }),
        );
        user = await choose("Reset votes");
        await user.click(
            within(screen.getByRole("dialog", { name: "Reset the votes in Stop?" })).getByRole("button", {
                name: "Reset votes",
            }),
        );

        // then
        expect(reset.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: STOP }, expect.anything());
    });

    it("opens the column's vote limit", async () => {
        // when
        renderMenu(makeBoardSnapshot());
        await choose("Vote limit…");

        // then
        expect(screen.getByRole("dialog", { name: "Vote limit for Stop" })).toBeInTheDocument();
    });

    it("says why a facilitation command was refused in the board's own words", () => {
        // given
        facilitation.useHideColumn.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "column hidden", code: "column_hidden" }),
            }),
        );

        // when
        renderMenu(makeBoardSnapshot());

        // then
        expect(
            screen.getByText(
                "That column is hidden from participants, so nothing in it can change until it is shown again.",
            ),
        ).toBeInTheDocument();
    });

    it("works on the first column too, with no move left", async () => {
        // when
        renderMenu(makeBoardSnapshot(), START);
        const items = await itemsOf("Start");

        // then
        expect(items).not.toContain("Move left");
        expect(items).toContain("Move right");
    });
});
