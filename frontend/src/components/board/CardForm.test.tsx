import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardMembership } from "../../hooks/board/useBoardMembership";
import { BOARD_ID, makeApiError, makePerson } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { CardForm, type CardInitial } from "./CardForm";

const { useBoardMembership } = vi.hoisted(() => ({ useBoardMembership: vi.fn() }));

vi.mock("../../hooks/board/useBoardMembership", () => ({ useBoardMembership }));

const sam = makePerson({ user_id: "u1", display_name: "Sam" });
const ali = makePerson({ user_id: "u3", display_name: "Ali Khan" });

interface FormOptions {
    initial?: CardInitial;
    pending?: boolean;
    error?: Error | null;
}

function membership(overrides: Partial<BoardMembership> = {}): BoardMembership {
    return { people: [sam, ali], member: true, failure: null, retry: vi.fn(), ...overrides };
}

function renderForm({
    initial = { body: "", colour: null, mentions: [] },
    pending = false,
    error = null,
}: FormOptions = {}) {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();

    renderWithProviders(
        <CardForm
            boardId={BOARD_ID}
            label="Add a card to Start"
            initial={initial}
            submitLabel="Add card"
            pending={pending}
            error={error}
            onSubmit={onSubmit}
            onCancel={onCancel}
        />,
    );

    return { onSubmit, onCancel };
}

beforeEach(() => {
    vi.clearAllMocks();
    useBoardMembership.mockReturnValue(membership());
});

describe("CardForm", () => {
    it("hands over the text and the chosen colour", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderForm();

        // when
        await user.type(screen.getByLabelText("Card text"), "Ship smaller changes");
        await user.selectOptions(screen.getByLabelText("Card colour"), "Teal");
        await user.click(screen.getByRole("button", { name: "Add card" }));

        // then
        expect(onSubmit).toHaveBeenCalledExactlyOnceWith({
            body: "Ship smaller changes",
            colour: "teal",
            mentions: [],
        });
    });

    it("mentions a member picked from the @ list with the keyboard", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderForm();

        // when
        await user.type(screen.getByRole("combobox", { name: "Card text" }), "Thanks @al");
        await user.keyboard("{Enter}");
        await user.type(screen.getByRole("combobox", { name: "Card text" }), "for the demo");
        await user.click(screen.getByRole("button", { name: "Add card" }));

        // then
        expect(onSubmit).toHaveBeenCalledExactlyOnceWith({
            body: "Thanks @Ali Khan for the demo",
            colour: null,
            mentions: ["u3"],
        });
    });

    it("lists only the members whose names match what follows the @", async () => {
        // given
        const user = userEvent.setup();
        renderForm();

        // when
        await user.type(screen.getByRole("combobox", { name: "Card text" }), "Ask @kh");

        // then
        const people = screen.getByRole("listbox", { name: "People to mention" });
        expect(
            within(people)
                .getAllByRole("option")
                .map(option => option.textContent),
        ).toEqual(["Ali Khan"]);
        expect(screen.getByRole("combobox", { name: "Card text" })).toHaveAttribute("aria-expanded", "true");
    });

    it("starts from the card it edits, and No colour means no colour", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderForm({ initial: { body: "Pair on reviews @Sam", colour: "red", mentions: [sam] } });

        // when
        const before = (screen.getByLabelText("Card text") as HTMLTextAreaElement).value;
        await user.selectOptions(screen.getByLabelText("Card colour"), "No colour");
        await user.click(screen.getByRole("button", { name: "Add card" }));

        // then
        expect(before).toBe("Pair on reviews @Sam");
        expect(onSubmit).toHaveBeenCalledExactlyOnceWith({
            body: "Pair on reviews @Sam",
            colour: null,
            mentions: ["u1"],
        });
    });

    it("offers a guest a plain text box with no @ list", async () => {
        // given
        useBoardMembership.mockReturnValue(membership({ people: null, member: false }));
        const user = userEvent.setup();
        const { onSubmit } = renderForm();

        // when
        await user.type(screen.getByRole("textbox", { name: "Card text" }), "Ask @Sa");
        await user.click(screen.getByRole("button", { name: "Add card" }));

        // then
        expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
        expect(onSubmit).toHaveBeenCalledExactlyOnceWith({ body: "Ask @Sa", colour: null, mentions: [] });
    });

    it("says why the people to mention could not be read, and tries again", async () => {
        // given
        const retry = vi.fn();
        useBoardMembership.mockReturnValue(
            membership({
                people: null,
                member: false,
                failure: makeApiError({ status: 503, message: "service unavailable" }),
                retry,
            }),
        );
        const user = userEvent.setup();
        renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(retry).toHaveBeenCalledOnce();
    });

    it("says the card is empty, ties it to the field and hands nothing over", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderForm();

        // when
        await user.type(screen.getByLabelText("Card text"), "   ");
        await user.click(screen.getByRole("button", { name: "Add card" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Write something on the card first.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByLabelText("Card text")).toBeInvalid();
        expect(onSubmit).not.toHaveBeenCalled();
    });

    it.each([
        { status: 400, message: "a card needs some text", code: "card_body_required", field: "Card text" },
        {
            status: 400,
            message: "a card can be at most 2000 characters",
            code: "card_body_too_long",
            field: "Card text",
        },
        { status: 400, message: "that is not a board colour", code: "invalid_colour", field: "Card colour" },
        {
            status: 400,
            message: "an action item, card or comment can mention at most 20 people",
            code: "too_many_mentions",
            field: "Card text",
        },
        {
            status: 400,
            message: "an assignee or a mention must be a member who can open the board",
            code: "not_board_member",
            field: "Card text",
        },
    ])("ties the server's refusal to its field: $code", ({ field, ...refusal }) => {
        // when
        renderForm({ error: makeApiError(refusal) });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText(field)).toHaveAccessibleDescription(expect.stringContaining(refusal.message));
    });

    it("shows any other refusal as it comes, without blaming a field", () => {
        // when
        renderForm({ error: makeApiError({ status: 409, message: "cards are locked", code: "cards_locked" }) });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("cards are locked");
        expect(screen.getByLabelText("Card text")).not.toHaveAttribute("aria-invalid");
    });

    it("holds both buttons while the card is saved", () => {
        // when
        renderForm({ pending: true });

        // then
        expect(screen.getByRole("button", { name: "Add card" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    });

    it("cancels", async () => {
        // given
        const user = userEvent.setup();
        const { onCancel } = renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onCancel).toHaveBeenCalledOnce();
    });
});
