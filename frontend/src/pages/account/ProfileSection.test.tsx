import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { ProfileSection } from "./ProfileSection";

const { useUpdateProfile } = vi.hoisted(() => ({ useUpdateProfile: vi.fn() }));

vi.mock("../../hooks/mutations/auth", () => ({ useUpdateProfile }));

const account = makeSessionUser({ display_name: "Sam" });

async function openDialog() {
    const user = userEvent.setup();
    renderWithProviders(<ProfileSection user={account} />, { user: account });
    await user.click(screen.getByRole("button", { name: "Change display name" }));

    return user;
}

beforeEach(() => {
    useUpdateProfile.mockReturnValue(makeMutationState());
});

describe("ProfileSection", () => {
    it("shows the current display name with a button to change it, and no field until asked", () => {
        // when
        renderWithProviders(<ProfileSection user={account} />, { user: account });

        // then
        expect(screen.getByText("Sam")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Change display name" })).toBeInTheDocument();
        expect(screen.queryByLabelText("Display name")).not.toBeInTheDocument();
    });

    it("opens a dialog that starts from the current display name", async () => {
        // when
        await openDialog();

        // then
        expect(screen.getByRole("dialog", { name: "Change display name" })).toBeInTheDocument();
        expect(screen.getByLabelText("Display name")).toHaveValue("Sam");
        expect(screen.getByLabelText("Display name")).toHaveAccessibleDescription("Up to 40 characters.");
        expect(screen.getByLabelText("Display name")).toHaveAttribute("maxlength", "40");
    });

    it("saves the new display name", async () => {
        // given
        const update = makeMutationState();
        useUpdateProfile.mockReturnValue(update);
        const user = await openDialog();

        // when
        await user.clear(screen.getByLabelText("Display name"));
        await user.type(screen.getByLabelText("Display name"), "Samira");
        await user.click(screen.getByRole("button", { name: "Save display name" }));

        // then
        expect(update.mutate).toHaveBeenCalledWith({ display_name: "Samira" }, expect.anything());
    });

    it("asks for a name before sending anything and ties the message to the field", async () => {
        // given
        const update = makeMutationState();
        useUpdateProfile.mockReturnValue(update);
        const user = await openDialog();

        // when
        await user.clear(screen.getByLabelText("Display name"));
        await user.type(screen.getByLabelText("Display name"), "   ");
        await user.click(screen.getByRole("button", { name: "Save display name" }));

        // then
        expect(screen.getByLabelText("Display name")).toHaveAccessibleDescription(
            "Up to 40 characters. Enter a display name.",
        );
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a display name.");
        expect(update.mutate).not.toHaveBeenCalled();
    });

    it("holds the save button while the name is being saved", async () => {
        // given
        useUpdateProfile.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("button", { name: "Save display name" })).toBeDisabled();
    });

    it("says the name was saved", () => {
        // given
        useUpdateProfile.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<ProfileSection user={account} />, { user: account });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Display name saved.");
    });

    it("ties a refused name to the field", async () => {
        // given
        const failure = makeApiError({ status: 400, message: "a display name is required" });
        useUpdateProfile.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("a display name is required");
        expect(screen.getByLabelText("Display name")).toHaveAttribute("aria-invalid", "true");
    });

    it("shows any other failure without blaming the field", async () => {
        // given
        const failure = makeApiError({ status: 403, message: "your account is locked" });
        useUpdateProfile.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("your account is locked");
        expect(screen.getByLabelText("Display name")).not.toHaveAttribute("aria-invalid");
    });
});
