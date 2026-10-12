import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { EmailSection } from "./EmailSection";

const { useSetUserEmail, useVerifyUserEmail, useUnverifyUserEmail } = vi.hoisted(() => ({
    useSetUserEmail: vi.fn(),
    useVerifyUserEmail: vi.fn(),
    useUnverifyUserEmail: vi.fn(),
}));

vi.mock("../../../hooks/mutations/admin", () => ({ useSetUserEmail, useVerifyUserEmail, useUnverifyUserEmail }));

const verified = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", email_verified: true });
const unverified = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", email_verified: false });

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    useSetUserEmail.mockReturnValue(makeMutationState());
    useVerifyUserEmail.mockReturnValue(makeMutationState());
    useUnverifyUserEmail.mockReturnValue(makeMutationState());
});

describe("EmailSection", () => {
    describe("changing the address", () => {
        it("stays away without the permission", () => {
            // given
            const canChange = false;

            // when
            renderWithProviders(<EmailSection user={verified} canChange={canChange} canVerify />);

            // then
            expect(screen.queryByRole("textbox", { name: "New email address" })).not.toBeInTheDocument();
            expect(screen.queryByRole("button", { name: "Change email address" })).not.toBeInTheDocument();
        });

        it("sends the new address, trimmed, and clears the field once the link is sent", async () => {
            // given
            const change = makeMutationState({ mutate: vi.fn(succeed) });
            useSetUserEmail.mockReturnValue(change);
            const user = userEvent.setup();
            renderWithProviders(<EmailSection user={verified} canChange canVerify={false} />);
            const field = screen.getByRole("textbox", { name: "New email address" });

            // when
            await user.type(field, " new@example.test ");
            await user.click(screen.getByRole("button", { name: "Change email address" }));

            // then
            expect(change.mutate).toHaveBeenCalledWith(
                { userId: "user-robin", email: "new@example.test" },
                expect.anything(),
            );
            expect(field).toHaveValue("");
        });

        it("asks for an address instead of sending an empty one", async () => {
            // given
            const change = makeMutationState();
            useSetUserEmail.mockReturnValue(change);
            const user = userEvent.setup();
            renderWithProviders(<EmailSection user={verified} canChange canVerify={false} />);

            // when
            await user.click(screen.getByRole("button", { name: "Change email address" }));

            // then
            expect(change.mutate).not.toHaveBeenCalled();
            expect(screen.getByRole("textbox", { name: "New email address" })).toHaveAccessibleDescription(
                expect.stringContaining("Enter the new email address."),
            );
            expect(screen.getByRole("alert")).toHaveTextContent("Enter the new email address.");
            expect(screen.getByRole("alert")).toHaveFocus();
        });

        it("says the link was sent", () => {
            // given
            useSetUserEmail.mockReturnValue(makeMutationState({ isSuccess: true }));

            // when
            renderWithProviders(<EmailSection user={verified} canChange canVerify={false} />);

            // then
            expect(screen.getByRole("status")).toHaveTextContent("Link sent. The address changes once it is used.");
        });

        it("shows the server's reason when the address was refused", () => {
            // given
            const error = makeApiError({ status: 409, message: "that email address is already in use" });
            useSetUserEmail.mockReturnValue(makeMutationState({ isError: true, error }));

            // when
            renderWithProviders(<EmailSection user={verified} canChange canVerify={false} />);

            // then
            expect(screen.getByRole("alert")).toHaveTextContent("that email address is already in use");
            expect(screen.getByRole("alert")).toHaveFocus();
        });

        it("says plainly that nothing was sent when the site cannot send email", () => {
            // given
            const error = makeApiError({
                status: 503,
                message: "email is not available on this site",
                code: "email_unavailable",
            });
            useSetUserEmail.mockReturnValue(makeMutationState({ isError: true, error }));

            // when
            renderWithProviders(<EmailSection user={verified} canChange canVerify={false} />);

            // then
            expect(screen.getByRole("alert")).toHaveTextContent(
                "This site cannot send email yet, so nothing was sent.",
            );
            expect(screen.queryByRole("status")).not.toBeInTheDocument();
        });
    });

    describe("verification", () => {
        it("stays away without the permission", () => {
            // given
            const canVerify = false;

            // when
            renderWithProviders(<EmailSection user={unverified} canChange canVerify={canVerify} />);

            // then
            expect(screen.queryByRole("button", { name: "Mark verified" })).not.toBeInTheDocument();
            expect(screen.queryByRole("button", { name: "Mark unverified" })).not.toBeInTheDocument();
        });

        it("marks an unverified address verified straight away", async () => {
            // given
            const verify = makeMutationState();
            useVerifyUserEmail.mockReturnValue(verify);
            const user = userEvent.setup();
            renderWithProviders(<EmailSection user={unverified} canChange={false} canVerify />);

            // when
            await user.click(screen.getByRole("button", { name: "Mark verified" }));

            // then
            expect(verify.mutate).toHaveBeenCalledWith({ userId: "user-robin" });
        });

        it("asks before marking a verified address unverified, warning that it signs the user out", async () => {
            // given
            const unverify = makeMutationState({ mutate: vi.fn(succeed) });
            useUnverifyUserEmail.mockReturnValue(unverify);
            const user = userEvent.setup();
            renderWithProviders(<EmailSection user={verified} canChange={false} canVerify />);

            // when
            await user.click(screen.getByRole("button", { name: "Mark unverified" }));
            const dialog = screen.getByRole("dialog", { name: "Mark email unverified" });
            await user.click(within(dialog).getByRole("button", { name: "Mark unverified" }));

            // then
            expect(dialog).toHaveTextContent(
                "Robin is signed out everywhere and cannot sign in until they verify the address again.",
            );
            expect(unverify.mutate).toHaveBeenCalledWith({ userId: "user-robin" }, expect.anything());
            expect(screen.queryByRole("dialog", { name: "Mark email unverified" })).not.toBeInTheDocument();
        });

        it("leaves the address alone when the check is cancelled", async () => {
            // given
            const unverify = makeMutationState();
            useUnverifyUserEmail.mockReturnValue(unverify);
            const user = userEvent.setup();
            renderWithProviders(<EmailSection user={verified} canChange={false} canVerify />);

            // when
            await user.click(screen.getByRole("button", { name: "Mark unverified" }));
            await user.click(screen.getByRole("button", { name: "Cancel" }));

            // then
            expect(unverify.mutate).not.toHaveBeenCalled();
            expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        });

        it("keeps the check open with the server's reason when marking unverified fails", async () => {
            // given
            const error = makeApiError({ status: 403, message: "this user cannot be modified" });
            useUnverifyUserEmail.mockReturnValue(makeMutationState({ isError: true, error }));
            const user = userEvent.setup();
            renderWithProviders(<EmailSection user={verified} canChange={false} canVerify />);

            // when
            await user.click(screen.getByRole("button", { name: "Mark unverified" }));

            // then
            expect(screen.getByRole("dialog", { name: "Mark email unverified" })).toHaveTextContent(
                "this user cannot be modified",
            );
        });

        it("shows the server's reason when marking verified fails", () => {
            // given
            const error = makeApiError({ status: 404, message: "user not found" });
            useVerifyUserEmail.mockReturnValue(makeMutationState({ isError: true, error }));

            // when
            renderWithProviders(<EmailSection user={unverified} canChange={false} canVerify />);

            // then
            expect(screen.getByRole("alert")).toHaveTextContent("user not found");
        });

        it("says when the address was marked verified or unverified", () => {
            // given
            useVerifyUserEmail.mockReturnValue(makeMutationState({ isSuccess: true }));
            useUnverifyUserEmail.mockReturnValue(makeMutationState({ isSuccess: true }));

            // when
            renderWithProviders(<EmailSection user={verified} canChange={false} canVerify />);

            // then
            const notices = screen.getAllByRole("status").map(notice => notice.textContent);
            expect(notices).toEqual(["Email marked verified.", "Email marked unverified."]);
        });
    });
});
