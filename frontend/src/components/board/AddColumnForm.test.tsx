import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeApiError, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { AddColumnForm } from "./AddColumnForm";

const { useCreateColumn } = vi.hoisted(() => ({ useCreateColumn: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useCreateColumn }));

beforeEach(() => {
    vi.clearAllMocks();
    useCreateColumn.mockReturnValue(makeMutationState());
});

describe("AddColumnForm", () => {
    it("adds a grey column unless another colour is picked, then closes", async () => {
        // given
        const create = makeMutationState();
        create.mutate.mockImplementation((_variables, options) => options.onSuccess());
        useCreateColumn.mockReturnValue(create);
        const user = userEvent.setup();
        renderWithProviders(<AddColumnForm boardId={BOARD_ID} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a column" }));
        await user.type(screen.getByLabelText("Column title"), "Kudos");
        await user.click(screen.getByRole("button", { name: "Add column" }));

        // then
        expect(useCreateColumn).toHaveBeenCalledWith(BOARD_ID);
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { title: "Kudos", description: "", colour: "grey" },
            expect.anything(),
        );
        expect(screen.queryByRole("form")).not.toBeInTheDocument();
    });

    it("shows a refused column in the form", async () => {
        // given
        useCreateColumn.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "the board is read only", code: "read_only" }),
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(<AddColumnForm boardId={BOARD_ID} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a column" }));

        // then
        expect(screen.getByRole("form", { name: "Add a column" })).toHaveTextContent("the board is read only");
    });
});
