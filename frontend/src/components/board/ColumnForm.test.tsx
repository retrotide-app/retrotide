import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeApiError } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { ColumnForm, type ColumnFields } from "./ColumnForm";

interface FormOptions {
    initial?: ColumnFields;
    pending?: boolean;
    error?: Error | null;
}

function renderForm({
    initial = { title: "", description: "", colour: "grey" },
    pending = false,
    error = null,
}: FormOptions = {}) {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();

    renderWithProviders(
        <ColumnForm
            label="Add a column"
            initial={initial}
            submitLabel="Add column"
            pending={pending}
            error={error}
            onSubmit={onSubmit}
            onCancel={onCancel}
        />,
    );

    return { onSubmit, onCancel };
}

describe("ColumnForm", () => {
    it("hands over the title, description and colour", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderForm();

        // when
        await user.type(screen.getByLabelText("Column title"), "Ideas");
        await user.type(screen.getByLabelText("Description"), "Anything new to try");
        await user.selectOptions(screen.getByLabelText("Column colour"), "Purple");
        await user.click(screen.getByRole("button", { name: "Add column" }));

        // then
        expect(onSubmit).toHaveBeenCalledExactlyOnceWith({
            title: "Ideas",
            description: "Anything new to try",
            colour: "purple",
        });
    });

    it("starts from the column it edits", () => {
        // when
        renderForm({ initial: { title: "Start", description: "What should we begin?", colour: "green" } });

        // then
        expect(screen.getByLabelText("Column title")).toHaveValue("Start");
        expect(screen.getByLabelText("Description")).toHaveValue("What should we begin?");
        expect(screen.getByLabelText("Column colour")).toHaveValue("green");
    });

    it("asks for a title and hands nothing over without one", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Add column" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a column title.");
        expect(screen.getByLabelText("Column title")).toBeInvalid();
        expect(onSubmit).not.toHaveBeenCalled();
    });

    it.each([
        { status: 400, message: "a column needs a title", code: "column_title_required", field: "Column title" },
        {
            status: 400,
            message: "a column title can be at most 80 characters",
            code: "column_title_too_long",
            field: "Column title",
        },
        {
            status: 400,
            message: "a column description can be at most 200 characters",
            code: "column_description_too_long",
            field: "Description",
        },
        { status: 400, message: "that is not a board colour", code: "invalid_colour", field: "Column colour" },
    ])("ties the server's refusal to its field: $code", ({ field, ...refusal }) => {
        // when
        renderForm({ error: makeApiError(refusal) });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText(field)).toHaveAccessibleDescription(expect.stringContaining(refusal.message));
    });

    it("shows any other refusal as it comes, without blaming a field", () => {
        // when
        renderForm({ error: makeApiError({ status: 409, message: "the board is read only", code: "read_only" }) });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the board is read only");
        expect(screen.getByLabelText("Column title")).not.toHaveAttribute("aria-invalid");
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

    it("holds both buttons while the column is saved", () => {
        // when
        renderForm({ pending: true });

        // then
        expect(screen.getByRole("button", { name: "Add column" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    });
});
