import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeApiError } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { ReasonDialog } from "./ReasonDialog";

interface Handlers {
    onConfirm?: () => void;
    onCancel?: () => void;
    busy?: boolean;
    error?: Error | null;
}

function renderDialog({ onConfirm = vi.fn(), onCancel = vi.fn(), busy = false, error = null }: Handlers = {}) {
    return renderWithProviders(
        <ReasonDialog
            title="Ban user"
            body="Robin is signed out everywhere."
            confirmLabel="Ban user"
            busy={busy}
            error={error}
            onConfirm={onConfirm}
            onCancel={onCancel}
        />,
    );
}

describe("ReasonDialog", () => {
    it("explains the action and asks for a reason", () => {
        // given
        const title = "Ban user";

        // when
        renderDialog();

        // then
        const dialog = screen.getByRole("dialog", { name: title });
        expect(dialog).toHaveTextContent("Robin is signed out everywhere.");
        expect(screen.getByRole("textbox", { name: "Reason" })).toBeInTheDocument();
    });

    it("hands over the trimmed reason", async () => {
        // given
        const onConfirm = vi.fn();
        const user = userEvent.setup();
        renderDialog({ onConfirm });

        // when
        await user.type(screen.getByRole("textbox", { name: "Reason" }), "  spam  ");
        await user.click(screen.getByRole("button", { name: "Ban user" }));

        // then
        expect(onConfirm).toHaveBeenCalledWith("spam");
    });

    it("asks for a reason instead of going ahead without one", async () => {
        // given
        const onConfirm = vi.fn();
        const user = userEvent.setup();
        renderDialog({ onConfirm });

        // when
        await user.click(screen.getByRole("button", { name: "Ban user" }));

        // then
        expect(onConfirm).not.toHaveBeenCalled();
        expect(screen.getByRole("textbox", { name: "Reason" })).toHaveAccessibleDescription("Give a reason.");
        expect(screen.getByRole("textbox", { name: "Reason" })).toBeInvalid();
        expect(screen.getByRole("alert")).toHaveTextContent("Give a reason.");
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("shows the server's reason when the action failed", () => {
        // given
        const error = makeApiError({ status: 403, message: "this user cannot be modified" });

        // when
        renderDialog({ error });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("this user cannot be modified");
    });

    it("moves focus to the server's reason once a running action fails", () => {
        // given
        const { rerender } = renderDialog({ busy: true });
        const error = makeApiError({ status: 403, message: "this user cannot be modified" });

        // when
        rerender(
            <ReasonDialog
                title="Ban user"
                body="Robin is signed out everywhere."
                confirmLabel="Ban user"
                busy={false}
                error={error}
                onConfirm={vi.fn()}
                onCancel={vi.fn()}
            />,
        );

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("this user cannot be modified");
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("holds its buttons while the action is running", () => {
        // given
        const busy = true;

        // when
        renderDialog({ busy });

        // then
        expect(screen.getByRole("button", { name: "Ban user" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    });

    it("backs out when cancelled", async () => {
        // given
        const onCancel = vi.fn();
        const user = userEvent.setup();
        renderDialog({ onCancel });

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onCancel).toHaveBeenCalledOnce();
    });
});
