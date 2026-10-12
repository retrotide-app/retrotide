import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BOARD_ID, makeApiError, makeBoardView, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { BoardDescription } from "./BoardDescription";

const { useDescribeBoard } = vi.hoisted(() => ({ useDescribeBoard: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useDescribeBoard }));

const described = makeBoardView({ context: "Sprint 12, the release sprint" });

beforeEach(() => {
    vi.clearAllMocks();
    useDescribeBoard.mockReturnValue(makeMutationState());
});

describe("BoardDescription", () => {
    it("shows the description to everyone, with no way to edit it without board.manage", () => {
        // when
        renderWithProviders(<BoardDescription board={described} editable={false} />);

        // then
        expect(screen.getByText("Sprint 12, the release sprint")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Edit description" })).not.toBeInTheDocument();
    });

    it("shows nothing when there is no description and the viewer cannot add one", () => {
        // when
        const { container } = renderWithProviders(<BoardDescription board={makeBoardView()} editable={false} />);

        // then
        expect(container).toBeEmptyDOMElement();
    });

    it("invites a manager to add a description when there is none", () => {
        // when
        renderWithProviders(<BoardDescription board={makeBoardView()} editable />);

        // then
        expect(screen.getByText("No description yet.")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Edit description" })).toBeInTheDocument();
    });

    it("saves an edited description and closes the form", async () => {
        // given
        const saving = makeMutationState();
        saving.mutate.mockImplementation((_variables, options) => options.onSuccess());
        useDescribeBoard.mockReturnValue(saving);
        const user = userEvent.setup();
        renderWithProviders(<BoardDescription board={described} editable />);

        // when
        await user.click(screen.getByRole("button", { name: "Edit description" }));
        const field = screen.getByLabelText("Description");
        await user.clear(field);
        await user.type(field, "Release retro");
        await user.click(screen.getByRole("button", { name: "Save description" }));

        // then
        expect(useDescribeBoard).toHaveBeenCalledWith(BOARD_ID);
        expect(saving.mutate).toHaveBeenCalledExactlyOnceWith({ context: "Release retro" }, expect.anything());
        expect(screen.queryByRole("form")).not.toBeInTheDocument();
    });

    it("clears the description when the field is emptied", async () => {
        // given
        const saving = makeMutationState();
        useDescribeBoard.mockReturnValue(saving);
        const user = userEvent.setup();
        renderWithProviders(<BoardDescription board={described} editable />);

        // when
        await user.click(screen.getByRole("button", { name: "Edit description" }));
        await user.clear(screen.getByLabelText("Description"));
        await user.click(screen.getByRole("button", { name: "Save description" }));

        // then
        expect(saving.mutate).toHaveBeenCalledExactlyOnceWith({ context: "" }, expect.anything());
    });

    it("ties a refused description to the field", async () => {
        // given
        const refusal = {
            status: 400,
            message: "a board description can be at most 500 characters",
            code: "board_context_too_long",
        };
        useDescribeBoard.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));
        const user = userEvent.setup();
        renderWithProviders(<BoardDescription board={described} editable />);

        // when
        await user.click(screen.getByRole("button", { name: "Edit description" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Description")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
    });

    it("shows any other refusal as it comes, without blaming the field", async () => {
        // given
        useDescribeBoard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "the board is read only", code: "read_only" }),
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(<BoardDescription board={described} editable />);

        // when
        await user.click(screen.getByRole("button", { name: "Edit description" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the board is read only");
        expect(screen.getByLabelText("Description")).not.toHaveAttribute("aria-invalid");
    });
});
