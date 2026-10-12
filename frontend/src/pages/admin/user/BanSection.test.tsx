import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { BanSection } from "./BanSection";

const { useBanUser, useUnbanUser } = vi.hoisted(() => ({ useBanUser: vi.fn(), useUnbanUser: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useBanUser, useUnbanUser }));

const active = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", banned: false });
const banned = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", banned: true, ban_reason: "spam" });

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    useBanUser.mockReturnValue(makeMutationState());
    useUnbanUser.mockReturnValue(makeMutationState());
});

describe("BanSection", () => {
    it("asks for a reason before banning, then bans with it", async () => {
        // given
        const ban = makeMutationState({ mutate: vi.fn(succeed) });
        useBanUser.mockReturnValue(ban);
        const user = userEvent.setup();
        renderWithProviders(<BanSection user={active} />);

        // when
        await user.click(screen.getByRole("button", { name: "Ban user" }));
        const dialog = screen.getByRole("dialog", { name: "Ban user" });
        await user.type(within(dialog).getByRole("textbox", { name: "Reason" }), "spam");
        await user.click(within(dialog).getByRole("button", { name: "Ban user" }));

        // then
        expect(dialog).toHaveTextContent("Robin is signed out everywhere and cannot sign in until you unban them.");
        expect(ban.mutate).toHaveBeenCalledWith({ userId: "user-robin", reason: "spam" }, expect.anything());
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("leaves the user alone when the ban is cancelled", async () => {
        // given
        const ban = makeMutationState();
        useBanUser.mockReturnValue(ban);
        const user = userEvent.setup();
        renderWithProviders(<BanSection user={active} />);

        // when
        await user.click(screen.getByRole("button", { name: "Ban user" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(ban.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when the ban fails", async () => {
        // given
        const error = makeApiError({ status: 403, message: "this user cannot be modified" });
        useBanUser.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderWithProviders(<BanSection user={active} />);

        // when
        await user.click(screen.getByRole("button", { name: "Ban user" }));

        // then
        expect(screen.getByRole("dialog", { name: "Ban user" })).toHaveTextContent("this user cannot be modified");
    });

    it("says the user was banned", () => {
        // given
        useBanUser.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<BanSection user={banned} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("User banned.");
    });

    it("unbans a banned user straight away", async () => {
        // given
        const unban = makeMutationState();
        useUnbanUser.mockReturnValue(unban);
        const user = userEvent.setup();
        renderWithProviders(<BanSection user={banned} />);

        // when
        await user.click(screen.getByRole("button", { name: "Unban user" }));

        // then
        expect(unban.mutate).toHaveBeenCalledWith({ userId: "user-robin" });
        expect(screen.queryByRole("button", { name: "Ban user" })).not.toBeInTheDocument();
    });

    it("shows the server's reason when the unban fails", () => {
        // given
        const error = makeApiError({ status: 500, message: "failed to unban the user" });
        useUnbanUser.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderWithProviders(<BanSection user={banned} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to unban the user");
    });

    it("says the user was unbanned", () => {
        // given
        useUnbanUser.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<BanSection user={active} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("User unbanned.");
    });
});
