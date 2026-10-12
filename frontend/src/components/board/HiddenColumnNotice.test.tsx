import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeApiError, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { HiddenColumnNotice } from "./HiddenColumnNotice";

const { useShowColumn } = vi.hoisted(() => ({ useShowColumn: vi.fn() }));

vi.mock("../../hooks/mutations/facilitation", () => ({ useShowColumn }));

const COLUMN_ID = "00000000-0000-4000-8000-000000000c02";

let show = makeMutationState();

beforeEach(() => {
    vi.clearAllMocks();
    show = makeMutationState();
    useShowColumn.mockReturnValue(show);
});

describe("HiddenColumnNotice", () => {
    it("says the column is hidden from participants", () => {
        // when
        renderWithProviders(<HiddenColumnNotice boardId={BOARD_ID} columnId={COLUMN_ID} onShowForMe={vi.fn()} />);

        // then
        expect(screen.getByText("Hidden from participants")).toBeInTheDocument();
    });

    it("shows the column to this viewer alone", async () => {
        // given
        const user = userEvent.setup();
        const onShowForMe = vi.fn();
        renderWithProviders(<HiddenColumnNotice boardId={BOARD_ID} columnId={COLUMN_ID} onShowForMe={onShowForMe} />);

        // when
        await user.click(screen.getByRole("button", { name: "Show for me" }));

        // then
        expect(onShowForMe).toHaveBeenCalledOnce();
        expect(show.mutate).not.toHaveBeenCalled();
    });

    it("shows the column to everyone", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<HiddenColumnNotice boardId={BOARD_ID} columnId={COLUMN_ID} onShowForMe={vi.fn()} />);

        // when
        await user.click(screen.getByRole("button", { name: "Show to everyone" }));

        // then
        expect(useShowColumn).toHaveBeenCalledWith(BOARD_ID);
        expect(show.mutate).toHaveBeenCalledExactlyOnceWith({ columnId: COLUMN_ID });
    });

    it("says why showing the column was refused", () => {
        // given
        useShowColumn.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 403, message: "your role on this board does not allow that" }),
            }),
        );

        // when
        renderWithProviders(<HiddenColumnNotice boardId={BOARD_ID} columnId={COLUMN_ID} onShowForMe={vi.fn()} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("your role on this board does not allow that");
    });
});
