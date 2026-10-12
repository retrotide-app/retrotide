import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardMembership } from "../../hooks/board/useBoardMembership";
import { BOARD_ID, makeApiError, makeColumn, makeMutationState, makePerson } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { AddCardForm } from "./AddCardForm";

const { useCreateCard } = vi.hoisted(() => ({ useCreateCard: vi.fn() }));

const { useBoardMembership } = vi.hoisted(() => ({ useBoardMembership: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useCreateCard }));
vi.mock("../../hooks/board/useBoardMembership", () => ({ useBoardMembership }));

const start = makeColumn();

const sam = makePerson({ user_id: "u1", display_name: "Sam" });

function membership(overrides: Partial<BoardMembership> = {}): BoardMembership {
    return { people: [sam], member: true, failure: null, retry: vi.fn(), ...overrides };
}

beforeEach(() => {
    vi.clearAllMocks();
    useCreateCard.mockReturnValue(makeMutationState());
    useBoardMembership.mockReturnValue(membership());
});

describe("AddCardForm", () => {
    it("adds a card to the column and clears the form for the next one", async () => {
        // given
        const create = makeMutationState();
        create.mutate.mockImplementation((_variables, options) => options.onSuccess());
        useCreateCard.mockReturnValue(create);
        const user = userEvent.setup();
        renderWithProviders(<AddCardForm boardId={BOARD_ID} column={start} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a card" }));
        await user.type(screen.getByLabelText("Card text"), "Demo every Friday");
        await user.click(screen.getByRole("button", { name: "Add card" }));

        // then
        expect(useCreateCard).toHaveBeenCalledWith(BOARD_ID);
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { column_id: start.id, body: "Demo every Friday", colour: null, mentions: [] },
            expect.anything(),
        );
        expect(screen.getByRole("form", { name: "Add a card to Start" })).toBeInTheDocument();
        expect(screen.getByLabelText("Card text")).toHaveValue("");
    });

    it("mentions a member picked with @, and Enter picks the person before it adds the card", async () => {
        // given
        const create = makeMutationState();
        useCreateCard.mockReturnValue(create);
        const user = userEvent.setup();
        renderWithProviders(<AddCardForm boardId={BOARD_ID} column={start} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a card" }));
        const text = screen.getByRole("combobox", { name: "Card text" });
        await user.type(text, "Thanks @s{Enter}");
        const sentOnPick = create.mutate.mock.calls.length;
        await user.type(text, "for the demo{Enter}");

        // then
        expect(useBoardMembership).toHaveBeenCalledWith(BOARD_ID, true);
        expect(sentOnPick).toBe(0);
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { column_id: start.id, body: "Thanks @Sam for the demo", colour: null, mentions: ["u1"] },
            expect.anything(),
        );
    });

    it("sends nobody whose name was picked and then deleted from the text", async () => {
        // given
        const create = makeMutationState();
        useCreateCard.mockReturnValue(create);
        const user = userEvent.setup();
        renderWithProviders(<AddCardForm boardId={BOARD_ID} column={start} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a card" }));
        const text = screen.getByRole("combobox", { name: "Card text" });
        await user.type(text, "Thanks @s{Enter}");
        await user.clear(text);
        await user.type(text, "Thanks all{Enter}");

        // then
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { column_id: start.id, body: "Thanks all", colour: null, mentions: [] },
            expect.anything(),
        );
    });

    it("asks for no people while the form is closed", () => {
        // when
        renderWithProviders(<AddCardForm boardId={BOARD_ID} column={start} />);

        // then
        expect(useBoardMembership).not.toHaveBeenCalled();
    });

    it("shows a refused card in the form", async () => {
        // given
        useCreateCard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "join the board first", code: "not_participant" }),
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(<AddCardForm boardId={BOARD_ID} column={start} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a card" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("join the board first");
    });

    it("closes the form on cancel", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<AddCardForm boardId={BOARD_ID} column={start} />);

        // when
        await user.click(screen.getByRole("button", { name: "Add a card" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(screen.queryByRole("form")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Add a card" })).toBeInTheDocument();
    });
});
