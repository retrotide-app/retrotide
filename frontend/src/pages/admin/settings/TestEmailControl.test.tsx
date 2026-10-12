import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { TestEmailControl } from "./TestEmailControl";

const { useSendTestEmail } = vi.hoisted(() => ({ useSendTestEmail: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useSendTestEmail }));

const sam = makeSessionUser({ email: "sam@example.test" });

function renderControl(unsaved: boolean, saveThen = vi.fn(), saving = false) {
    renderWithProviders(<TestEmailControl unsaved={unsaved} saving={saving} saveThen={saveThen} />, { user: sam });

    return saveThen;
}

beforeEach(() => {
    useSendTestEmail.mockReturnValue(makeMutationState());
});

describe("TestEmailControl", () => {
    it("says where the test email goes", () => {
        // when
        renderControl(false);

        // then
        expect(screen.getByText(/Sends a test email to/)).toHaveTextContent("Sends a test email to sam@example.test.");
    });

    it("sends the test email straight away when nothing is unsaved", async () => {
        // given
        const test = makeMutationState();
        useSendTestEmail.mockReturnValue(test);
        const user = userEvent.setup();
        const saveThen = renderControl(false);

        // when
        await user.click(screen.getByRole("button", { name: "Send test email" }));

        // then
        expect(test.mutate).toHaveBeenCalledOnce();
        expect(saveThen).not.toHaveBeenCalled();
    });

    it("saves the changes on screen first, then sends the test email", async () => {
        // given
        const test = makeMutationState();
        useSendTestEmail.mockReturnValue(test);
        const user = userEvent.setup();
        const saveThen = renderControl(
            true,
            vi.fn((next: () => void) => next()),
        );

        // when
        await user.click(screen.getByRole("button", { name: "Save and send test email" }));

        // then
        expect(screen.getByText(/Sends a test email to/)).toHaveTextContent("Your changes are saved first.");
        expect(saveThen).toHaveBeenCalledOnce();
        expect(test.mutate).toHaveBeenCalledOnce();
    });

    it("holds the button while the email is being sent or the settings save", () => {
        // given
        useSendTestEmail.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderControl(false);

        // then
        expect(screen.getByRole("button", { name: "Send test email" })).toBeDisabled();
    });

    it("says the email was sent", () => {
        // given
        useSendTestEmail.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderControl(false);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Test email sent. Check your inbox.");
    });

    it("shows the server's reason when the email could not be sent", () => {
        // given
        const error = makeApiError({ status: 503, message: "email is not available on this site" });
        useSendTestEmail.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderControl(false);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("email is not available on this site");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
