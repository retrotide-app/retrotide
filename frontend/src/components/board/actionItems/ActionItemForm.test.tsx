import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeApiError, makePerson } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { PersonView } from "../../../types/api";
import { ActionItemForm, type ActionItemInitial } from "./ActionItemForm";

const sam = makePerson({ user_id: "u1", display_name: "Sam" });
const samLee = makePerson({ user_id: "u2", display_name: "Sam Lee" });
const ali = makePerson({ user_id: "u3", display_name: "Ali Khan" });

const EMPTY: ActionItemInitial = { title: "", assignee: null, dueOn: null, mentions: [] };

interface FormOptions {
    initial?: ActionItemInitial;
    people?: readonly PersonView[] | null;
    error?: Error | null;
}

function renderForm({ initial = EMPTY, people = [sam, samLee, ali], error = null }: FormOptions = {}) {
    const onSubmit = vi.fn();
    renderWithProviders(
        <ActionItemForm
            label="Add an action item"
            initial={initial}
            people={people}
            submitLabel="Add"
            pending={false}
            error={error}
            onSubmit={onSubmit}
        />,
    );

    return onSubmit;
}

function titleField() {
    return screen.getByRole("combobox", { name: "Action item" });
}

describe("ActionItemForm", () => {
    it("sends the title, the assignee and the due date", async () => {
        // given
        const user = userEvent.setup();
        const onSubmit = renderForm();

        // when
        await user.type(titleField(), "Book the room");
        await user.selectOptions(screen.getByRole("combobox", { name: "Assignee" }), "Ali Khan");
        await user.type(screen.getByLabelText("Due date"), "2026-10-20");
        await user.click(screen.getByRole("button", { name: "Add" }));

        // then
        expect(onSubmit).toHaveBeenCalledWith({
            title: "Book the room",
            assignee_user_id: "u3",
            due_on: "2026-10-20",
            mentions: [],
        });
    });

    it("asks for a title before sending anything", async () => {
        // given
        const user = userEvent.setup();
        const onSubmit = renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Add" }));

        // then
        expect(onSubmit).not.toHaveBeenCalled();
        expect(screen.getByText("Write what needs doing first.")).toBeInTheDocument();
    });

    it("suggests organisation members after an @ and keeps the picked person as a mention", async () => {
        // given
        const user = userEvent.setup();
        const onSubmit = renderForm();

        // when
        await user.type(titleField(), "Ask @sa");
        const suggestions = screen.getByRole("listbox", { name: "People to mention" });

        // then
        expect(
            within(suggestions)
                .getAllByRole("option")
                .map(option => option.textContent),
        ).toEqual(["Sam", "Sam Lee"]);

        // when
        await user.click(within(suggestions).getByRole("option", { name: "Sam Lee" }));
        await user.type(titleField(), "about it");
        await user.click(screen.getByRole("button", { name: "Add" }));

        // then
        expect(onSubmit).toHaveBeenCalledWith(
            expect.objectContaining({ title: "Ask @Sam Lee about it", mentions: ["u2"] }),
        );
    });

    it("picks the highlighted person with the keyboard", async () => {
        // given
        const user = userEvent.setup();
        renderForm();

        // when
        await user.type(titleField(), "@");
        await user.keyboard("{ArrowDown}{Enter}");

        // then
        expect(titleField()).toHaveValue("@Sam Lee ");
        expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    });

    it("closes the suggestions on Escape", async () => {
        // given
        const user = userEvent.setup();
        renderForm();

        // when
        await user.type(titleField(), "@a");
        await user.keyboard("{Escape}");

        // then
        expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
        expect(titleField()).toHaveAttribute("aria-expanded", "false");
    });

    it("drops a mention whose name was deleted from the title", async () => {
        // given
        const user = userEvent.setup();
        const onSubmit = renderForm({ initial: { ...EMPTY, title: "Ask @Sam", mentions: [sam] } });

        // when
        await user.clear(titleField());
        await user.type(titleField(), "Ask nobody");
        await user.click(screen.getByRole("button", { name: "Add" }));

        // then
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ mentions: [] }));
    });

    it("offers no suggestions and no picker to someone who may not name members, and keeps what is set", async () => {
        // given
        const user = userEvent.setup();
        const onSubmit = renderForm({
            people: null,
            initial: { title: "Ask @Sam", assignee: ali, dueOn: null, mentions: [sam] },
        });

        // when
        await user.type(titleField(), " @");
        await user.click(screen.getByRole("button", { name: "Add" }));

        // then
        expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
        expect(screen.getByLabelText("Assignee")).toHaveValue("Ali Khan");
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ assignee_user_id: "u3", mentions: ["u1"] }));
    });

    it("keeps an assignee who is no longer on the list", () => {
        // when
        renderForm({ people: [sam], initial: { ...EMPTY, title: "Ship it", assignee: ali } });

        // then
        expect(screen.getByRole("combobox", { name: "Assignee" })).toHaveValue("u3");
    });

    it("shows a refusal on the field it belongs to", () => {
        // when
        renderForm({
            error: makeApiError({
                status: 400,
                message: "that person cannot open this board",
                code: "not_board_member",
            }),
        });

        // then
        expect(screen.getByRole("combobox", { name: "Assignee" })).toHaveAccessibleDescription(
            "that person cannot open this board",
        );
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("shows any other refusal above the form", () => {
        // when
        renderForm({
            error: makeApiError({
                status: 409,
                message: "action items can be added, edited and marked only in the Actions stage",
                code: "wrong_stage",
            }),
        });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("only in the Actions stage");
    });
});
