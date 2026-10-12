import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { EmailSection } from "./EmailSection";

const { useSetEmail } = vi.hoisted(() => ({ useSetEmail: vi.fn() }));

vi.mock("../../hooks/mutations/auth", () => ({ useSetEmail }));

const account = makeSessionUser({ email: "sam@example.test" });

function succeeding() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
            options?.onSuccess?.();
        }),
    });
}

function renderSection(pendingEmail: string | null = null, onChangeRequested = vi.fn(), user = account) {
    renderWithProviders(
        <EmailSection user={user} pendingEmail={pendingEmail} onChangeRequested={onChangeRequested} />,
        { user },
    );

    return onChangeRequested;
}

async function openDialog() {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Change email address" }));

    return user;
}

function dialog() {
    return within(screen.getByRole("dialog", { name: "Change email address" }));
}

beforeEach(() => {
    useSetEmail.mockReturnValue(makeMutationState());
});

describe("EmailSection", () => {
    it("shows the address the account uses, with no form until asked", () => {
        // given
        const user = account;

        // when
        renderSection(null, vi.fn(), user);

        // then
        expect(screen.getByRole("region", { name: "Email address" })).toHaveTextContent(
            "Your email address is sam@example.test.",
        );
        expect(screen.queryByLabelText("New email address")).not.toBeInTheDocument();
    });

    it("mentions no pending change while none was asked for", () => {
        // given
        const pendingEmail = null;

        // when
        renderSection(pendingEmail);

        // then
        expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it("shows the pending address until its link is used", () => {
        // given
        const pendingEmail = "sam@new.example.test";

        // when
        renderSection(pendingEmail);

        // then
        expect(screen.getByRole("status")).toHaveTextContent(
            "We sent a link to sam@new.example.test. Your email address changes to it when you use the link.",
        );
    });

    it("opens a dialog with the new address field focused", async () => {
        // given
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByLabelText("New email address")).toHaveFocus();
        expect(dialog().getByLabelText("Current password")).toBeInTheDocument();
    });

    it("asks for the change with the new address and the current password", async () => {
        // given
        const change = makeMutationState();
        useSetEmail.mockReturnValue(change);
        renderSection();
        const user = await openDialog();

        // when
        await user.type(dialog().getByLabelText("New email address"), "  sam@new.example.test ");
        await user.type(dialog().getByLabelText("Current password"), "harbour-lights");
        await user.click(dialog().getByRole("button", { name: "Send link" }));

        // then
        expect(change.mutate).toHaveBeenCalledWith(
            { email: "sam@new.example.test", password: "harbour-lights" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("hands the new address up and closes the dialog once the link is sent", async () => {
        // given
        useSetEmail.mockReturnValue(succeeding());
        const onChangeRequested = renderSection();
        const user = await openDialog();

        // when
        await user.type(dialog().getByLabelText("New email address"), "sam@new.example.test");
        await user.type(dialog().getByLabelText("Current password"), "harbour-lights{Enter}");

        // then
        expect(onChangeRequested).toHaveBeenCalledWith("sam@new.example.test");
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("clears what was typed when the dialog is cancelled", async () => {
        // given
        const change = makeMutationState();
        useSetEmail.mockReturnValue(change);
        renderSection();
        const user = await openDialog();
        await user.type(dialog().getByLabelText("New email address"), "sam@new.example.test");

        // when
        await user.click(dialog().getByRole("button", { name: "Cancel" }));
        await user.click(screen.getByRole("button", { name: "Change email address" }));

        // then
        expect(dialog().getByLabelText("New email address")).toHaveValue("");
        expect(change.mutate).not.toHaveBeenCalled();
    });

    it("says what is missing before sending anything, ties each message to its field and moves focus to it", async () => {
        // given
        const change = makeMutationState();
        useSetEmail.mockReturnValue(change);
        renderSection();
        const user = await openDialog();

        // when
        await user.click(dialog().getByRole("button", { name: "Send link" }));

        // then
        expect(dialog().getByLabelText("New email address")).toHaveAccessibleDescription(
            "Enter the new email address.",
        );
        expect(dialog().getByLabelText("Current password")).toHaveAccessibleDescription("Enter your current password.");
        expect(dialog().getByRole("alert")).toHaveTextContent(
            "Enter the new email address. Enter your current password.",
        );
        expect(dialog().getByRole("alert")).toHaveFocus();
        expect(change.mutate).not.toHaveBeenCalled();
        expect(change.reset).toHaveBeenCalled();
    });

    it("holds the button while the change is being sent", async () => {
        // given
        useSetEmail.mockReturnValue(makeMutationState({ isPending: true }));
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByRole("button", { name: "Send link" })).toBeDisabled();
    });

    it("ties a wrong password to the password field", async () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "the current password is incorrect",
            code: "incorrect_password",
        });
        useSetEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByRole("alert")).toHaveTextContent("the current password is incorrect");
        expect(dialog().getByLabelText("Current password")).toHaveAttribute("aria-invalid", "true");
        expect(dialog().getByLabelText("Current password")).toHaveAccessibleDescription(
            "the current password is incorrect",
        );
        expect(dialog().getByLabelText("New email address")).not.toHaveAttribute("aria-invalid");
    });

    it("ties an address that is taken to the address field", async () => {
        // given
        const failure = makeApiError({ status: 409, message: "that email address is already in use" });
        useSetEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByLabelText("New email address")).toHaveAttribute("aria-invalid", "true");
        expect(dialog().getByLabelText("New email address")).toHaveAccessibleDescription(
            "that email address is already in use",
        );
        expect(dialog().getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
    });

    it("ties an address the server will not take to the address field", async () => {
        // given
        const failure = makeApiError({ status: 400, message: "that is already your email address" });
        useSetEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByLabelText("New email address")).toHaveAccessibleDescription(
            "that is already your email address",
        );
    });

    it("says plainly that nothing was sent when the site cannot send email, without blaming either field", async () => {
        // given
        const failure = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });
        useSetEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByRole("alert")).toHaveTextContent("This site cannot send email yet, so nothing was sent.");
        expect(dialog().getByLabelText("New email address")).not.toHaveAttribute("aria-invalid");
        expect(dialog().getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
    });

    it("shows a failure that is about neither field without blaming either", async () => {
        // given
        const failure = makeApiError({ status: 503, message: "email is not available on this site" });
        useSetEmail.mockReturnValue(makeMutationState({ isError: true, error: failure }));
        renderSection();

        // when
        await openDialog();

        // then
        expect(dialog().getByRole("alert")).toHaveTextContent("email is not available on this site");
        expect(dialog().getByLabelText("New email address")).not.toHaveAttribute("aria-invalid");
        expect(dialog().getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
    });

    it("asks an account with no password to set one first", () => {
        // given
        const user = makeSessionUser({ has_password: false });

        // when
        renderSection(null, vi.fn(), user);

        // then
        expect(screen.getByText("Set a password before you change your email address.")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Change email address" })).not.toBeInTheDocument();
    });
});
