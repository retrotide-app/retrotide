import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { DisplayNameSection } from "./DisplayNameSection";

const { useSetUserDisplayName } = vi.hoisted(() => ({ useSetUserDisplayName: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useSetUserDisplayName }));

const robin = makeAdminUserDetail({ id: "user-robin", display_name: "Robin" });

beforeEach(() => {
    useSetUserDisplayName.mockReturnValue(makeMutationState());
});

describe("DisplayNameSection", () => {
    it("starts from the user's current display name", () => {
        // given
        const user = robin;

        // when
        renderWithProviders(<DisplayNameSection user={user} />);

        // then
        expect(screen.getByRole("textbox", { name: "Display name" })).toHaveValue("Robin");
    });

    it("saves the new name, trimmed", async () => {
        // given
        const rename = makeMutationState();
        useSetUserDisplayName.mockReturnValue(rename);
        const user = userEvent.setup();
        renderWithProviders(<DisplayNameSection user={robin} />);
        const field = screen.getByRole("textbox", { name: "Display name" });

        // when
        await user.clear(field);
        await user.type(field, "  Robin Hood ");
        await user.click(screen.getByRole("button", { name: "Save display name" }));

        // then
        expect(rename.mutate).toHaveBeenCalledWith({ userId: "user-robin", display_name: "Robin Hood" });
    });

    it("asks for a name instead of saving an empty one, and moves focus to the message", async () => {
        // given
        const rename = makeMutationState();
        useSetUserDisplayName.mockReturnValue(rename);
        const user = userEvent.setup();
        renderWithProviders(<DisplayNameSection user={robin} />);

        // when
        await user.clear(screen.getByRole("textbox", { name: "Display name" }));
        await user.click(screen.getByRole("button", { name: "Save display name" }));

        // then
        expect(rename.mutate).not.toHaveBeenCalled();
        expect(rename.reset).toHaveBeenCalled();
        expect(screen.getByRole("textbox", { name: "Display name" })).toHaveAccessibleDescription(
            "Up to 40 characters. Enter a display name.",
        );
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a display name.");
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("stops the name at the length the server keeps", () => {
        // given
        const user = robin;

        // when
        renderWithProviders(<DisplayNameSection user={user} />);

        // then
        expect(screen.getByRole("textbox", { name: "Display name" })).toHaveAttribute("maxlength", "40");
    });

    it("holds the button while the name is saving", () => {
        // given
        useSetUserDisplayName.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<DisplayNameSection user={robin} />);

        // then
        expect(screen.getByRole("button", { name: "Save display name" })).toBeDisabled();
    });

    it("says the name was saved", () => {
        // given
        useSetUserDisplayName.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<DisplayNameSection user={robin} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Display name saved.");
    });

    it("shows the server's reason when the name was refused, and moves focus to it", () => {
        // given
        const error = makeApiError({ status: 403, message: "this user cannot be modified" });
        useSetUserDisplayName.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderWithProviders(<DisplayNameSection user={robin} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("this user cannot be modified");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
