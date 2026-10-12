import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeApiError } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { TeamNameDialog } from "./TeamNameDialog";

interface DialogOptions {
    open?: boolean;
    initialName?: string;
    busy?: boolean;
    error?: Error | null;
}

function renderDialog({ open = true, initialName, busy = false, error = null }: DialogOptions = {}) {
    const onSubmit = vi.fn();
    const onClose = vi.fn();

    renderWithProviders(
        <TeamNameDialog
            open={open}
            title="Rename team"
            submitLabel="Save name"
            initialName={initialName}
            busy={busy}
            error={error}
            onSubmit={onSubmit}
            onClose={onClose}
        />,
    );

    return { onSubmit, onClose };
}

describe("TeamNameDialog", () => {
    it("shows nothing while closed", () => {
        // when
        renderDialog({ open: false });

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("starts from the name it is given and submits the edited name", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderDialog({ initialName: "Crew" });

        // when
        const field = screen.getByLabelText("Team name");
        const before = (field as HTMLInputElement).value;
        await user.clear(field);
        await user.type(field, "Deck crew");
        await user.click(screen.getByRole("button", { name: "Save name" }));

        // then
        expect(screen.getByRole("dialog", { name: "Rename team" })).toBeInTheDocument();
        expect(before).toBe("Crew");
        expect(onSubmit).toHaveBeenCalledExactlyOnceWith("Deck crew");
    });

    it("says a name is needed, without submitting", async () => {
        // given
        const user = userEvent.setup();
        const { onSubmit } = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Save name" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a team name.");
        expect(screen.getByLabelText("Team name")).toBeInvalid();
        expect(onSubmit).not.toHaveBeenCalled();
    });

    it("ties a name refusal from the server to the field", () => {
        // given
        const error = makeApiError({ status: 409, message: "A team with that name exists.", code: "team_name_taken" });

        // when
        renderDialog({ error });

        // then
        expect(screen.getByLabelText("Team name")).toBeInvalid();
        expect(screen.getByLabelText("Team name")).toHaveAccessibleDescription(
            expect.stringContaining("A team with that name exists."),
        );
    });

    it("holds the submit button while busy", () => {
        // when
        renderDialog({ busy: true });

        // then
        expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
    });

    it("closes from the cancel button", async () => {
        // given
        const user = userEvent.setup();
        const { onClose } = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onClose).toHaveBeenCalledTimes(1);
    });
});
