import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useContext } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CardDeleteUndoContext } from "../hooks/board/useDeferredCardDelete";
import { makeMutationState } from "../test-utils/fixtures";
import { renderWithProviders } from "../test-utils/render";
import { CardDeleteUndoProvider } from "./CardDeleteUndoContext";

const boards = vi.hoisted(() => ({ useDeleteCard: vi.fn() }));

vi.mock("../hooks/mutations/boards", () => boards);

function Waiting() {
    const deferred = useContext(CardDeleteUndoContext);

    if (deferred === null) {
        return <p>No provider</p>;
    }

    return (
        <>
            <p>{`Waiting: ${deferred.waiting.join(", ")}`}</p>
            <button type="button" onClick={() => deferred.schedule("k1")}>
                Delete k1
            </button>
        </>
    );
}

beforeEach(() => {
    boards.useDeleteCard.mockReturnValue(makeMutationState());
});

describe("CardDeleteUndoProvider", () => {
    it("hands the board's deferred deletes to everything inside it", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <CardDeleteUndoProvider boardId="b1">
                <Waiting />
            </CardDeleteUndoProvider>,
        );

        // when
        await user.click(screen.getByRole("button", { name: "Delete k1" }));

        // then
        expect(boards.useDeleteCard).toHaveBeenCalledWith("b1");
        expect(screen.getByText("Waiting: k1")).toBeInTheDocument();
    });

    it("leaves a component outside it with no deferred deletes", () => {
        // when
        renderWithProviders(<Waiting />);

        // then
        expect(screen.getByText("No provider")).toBeInTheDocument();
    });
});
