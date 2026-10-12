import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeApiError, makeColumn, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { ColumnView } from "../../types/api";
import { ColumnVoteLimitDialog } from "./ColumnVoteLimitDialog";

const { useSetColumnVoteLimit } = vi.hoisted(() => ({ useSetColumnVoteLimit: vi.fn() }));

vi.mock("../../hooks/mutations/facilitation", () => ({ useSetColumnVoteLimit }));

const COLUMN_ID = "00000000-0000-4000-8000-000000000c01";

let limit = makeMutationState();

function renderDialog(column: ColumnView = makeColumn(), onClose = vi.fn()) {
    renderWithProviders(<ColumnVoteLimitDialog boardId={BOARD_ID} column={column} onClose={onClose} />);

    return onClose;
}

beforeEach(() => {
    vi.clearAllMocks();
    limit = makeMutationState();
    useSetColumnVoteLimit.mockReturnValue(limit);
});

describe("ColumnVoteLimitDialog", () => {
    it("starts with no column limit for a column without one", () => {
        // when
        renderDialog();

        // then
        expect(screen.getByRole("dialog", { name: "Vote limit for Start" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "No column limit" })).toBeChecked();
        expect(screen.getByLabelText("Votes per person in this column")).toBeDisabled();
    });

    it("sets a limit and closes once it is saved", async () => {
        // given
        const user = userEvent.setup();
        limit.mutate.mockImplementation((_variables, options) => options.onSuccess());
        const onClose = renderDialog();

        // when
        await user.click(screen.getByRole("switch", { name: "No column limit" }));
        await user.type(screen.getByLabelText("Votes per person in this column"), "3");
        await user.click(screen.getByRole("button", { name: "Save vote limit" }));

        // then
        expect(useSetColumnVoteLimit).toHaveBeenCalledWith(BOARD_ID);
        expect(limit.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: COLUMN_ID, maxVotes: 3 }, expect.anything());
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("lifts a column's limit", async () => {
        // given
        const user = userEvent.setup();
        renderDialog(makeColumn({ max_votes: 2 }));

        // when
        const before = screen.getByLabelText<HTMLInputElement>("Votes per person in this column").value;
        await user.click(screen.getByRole("switch", { name: "No column limit" }));
        await user.click(screen.getByRole("button", { name: "Save vote limit" }));

        // then
        expect(before).toBe("2");
        expect(limit.mutate).toHaveBeenCalledExactlyOnceWith(
            { columnId: COLUMN_ID, maxVotes: null },
            expect.anything(),
        );
    });

    it("refuses a limit outside 1 to 99 without saving", async () => {
        // given
        const user = userEvent.setup();
        renderDialog(makeColumn({ max_votes: 2 }));
        const field = screen.getByLabelText("Votes per person in this column");

        // when
        await user.clear(field);
        await user.type(field, "100");
        await user.click(screen.getByRole("button", { name: "Save vote limit" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a whole number from 1 to 99.");
        expect(limit.mutate).not.toHaveBeenCalled();
    });

    it("says why the board refused the limit", () => {
        // given
        useSetColumnVoteLimit.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 400, message: "bad limit", code: "invalid_column_vote_limit" }),
            }),
        );

        // when
        renderDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("A column vote limit must be between 1 and 99, or none.");
    });

    it("closes on cancel without saving", async () => {
        // given
        const user = userEvent.setup();
        const onClose = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
        expect(limit.mutate).not.toHaveBeenCalled();
    });
});
