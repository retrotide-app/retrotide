import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { TextField } from "../FormField/TextField";
import { FormDialog } from "./FormDialog";

function renderDialog(overrides: Partial<Parameters<typeof FormDialog>[0]> = {}) {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();

    renderWithProviders(
        <FormDialog open title="Rename team" submitLabel="Save" onSubmit={onSubmit} onCancel={onCancel} {...overrides}>
            <TextField label="Team name" defaultValue="Crew" />
        </FormDialog>,
    );

    return { onSubmit, onCancel };
}

describe("FormDialog", () => {
    it("renders nothing while it is closed", () => {
        // when
        renderDialog({ open: false });

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("shows its fields under its title and moves focus to the first one", () => {
        // when
        renderDialog();

        // then
        expect(screen.getByRole("dialog", { name: "Rename team" })).toBeInTheDocument();
        expect(screen.getByLabelText("Team name")).toHaveFocus();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    });

    it("moves focus to an error it opens with, ahead of the fields", () => {
        // when
        renderWithProviders(
            <FormDialog open title="Rename team" submitLabel="Save" onSubmit={vi.fn()} onCancel={vi.fn()}>
                <ErrorBanner message="the team is archived" takeFocus />
                <TextField label="Team name" defaultValue="Crew" />
            </FormDialog>,
        );

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("submits from the button", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Save" }));

        // then
        expect(onSubmit).toHaveBeenCalledOnce();
    });

    it("submits when Enter is pressed in a field", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderDialog();

        // when
        await user.type(screen.getByLabelText("Team name"), "{Enter}");

        // then
        expect(onSubmit).toHaveBeenCalledOnce();
    });

    it("cancels from the button, the close button and Escape", async () => {
        // given
        const user = userEvent.setup();
        const { onCancel } = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));
        await user.click(screen.getByRole("button", { name: "Close" }));
        await user.keyboard("{Escape}");

        // then
        expect(onCancel).toHaveBeenCalledTimes(3);
    });

    it("holds both buttons while busy and ignores a cancel", async () => {
        // given
        const user = userEvent.setup();
        const { onCancel, onSubmit } = renderDialog({ busy: true });

        // when
        await user.keyboard("{Escape}");
        await user.type(screen.getByLabelText("Team name"), "{Enter}");

        // then
        expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
        expect(onCancel).not.toHaveBeenCalled();
        expect(onSubmit).not.toHaveBeenCalled();
    });

    it("places extra actions between cancel and submit without submitting", async () => {
        // given
        const user = userEvent.setup();
        const other = vi.fn();
        const { onSubmit } = renderDialog({
            extraActions: (
                <button type="button" onClick={other}>
                    Something else
                </button>
            ),
        });

        // when
        await user.click(screen.getByRole("button", { name: "Something else" }));

        // then
        const labels = screen
            .getAllByRole("button")
            .map(button => button.textContent)
            .filter(label => label !== "✕");
        expect(labels).toEqual(["Cancel", "Something else", "Save"]);
        expect(other).toHaveBeenCalledOnce();
        expect(onSubmit).not.toHaveBeenCalled();
    });
});
