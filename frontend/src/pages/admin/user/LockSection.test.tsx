import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeAdminUserDetail, makeApiError, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { LockSection } from "./LockSection";

const { useLockUser, useUnlockUser } = vi.hoisted(() => ({ useLockUser: vi.fn(), useUnlockUser: vi.fn() }));

vi.mock("../../../hooks/mutations/admin", () => ({ useLockUser, useUnlockUser }));

const open = makeAdminUserDetail({ id: "user-robin", display_name: "Robin", locked: false });
const locked = makeAdminUserDetail({
    id: "user-robin",
    display_name: "Robin",
    locked: true,
    lock_reason: "chargeback",
});

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    useLockUser.mockReturnValue(makeMutationState());
    useUnlockUser.mockReturnValue(makeMutationState());
});

describe("LockSection", () => {
    it("asks for a reason before locking, then locks with it", async () => {
        // given
        const lock = makeMutationState({ mutate: vi.fn(succeed) });
        useLockUser.mockReturnValue(lock);
        const user = userEvent.setup();
        renderWithProviders(<LockSection user={open} />);

        // when
        await user.click(screen.getByRole("button", { name: "Lock user" }));
        const dialog = screen.getByRole("dialog", { name: "Lock user" });
        await user.type(within(dialog).getByRole("textbox", { name: "Reason" }), "chargeback");
        await user.click(within(dialog).getByRole("button", { name: "Lock user" }));

        // then
        expect(dialog).toHaveTextContent(
            "Robin can still sign in and read, but cannot make changes until you unlock them.",
        );
        expect(lock.mutate).toHaveBeenCalledWith({ userId: "user-robin", reason: "chargeback" }, expect.anything());
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("leaves the user alone when the lock is cancelled", async () => {
        // given
        const lock = makeMutationState();
        useLockUser.mockReturnValue(lock);
        const user = userEvent.setup();
        renderWithProviders(<LockSection user={open} />);

        // when
        await user.click(screen.getByRole("button", { name: "Lock user" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(lock.mutate).not.toHaveBeenCalled();
    });

    it("keeps the check open with the server's reason when the lock fails", async () => {
        // given
        const error = makeApiError({ status: 400, message: "a reason is required" });
        useLockUser.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderWithProviders(<LockSection user={open} />);

        // when
        await user.click(screen.getByRole("button", { name: "Lock user" }));

        // then
        expect(screen.getByRole("dialog", { name: "Lock user" })).toHaveTextContent("a reason is required");
    });

    it("says the user was locked", () => {
        // given
        useLockUser.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<LockSection user={locked} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("User locked.");
    });

    it("unlocks a locked user straight away", async () => {
        // given
        const unlock = makeMutationState();
        useUnlockUser.mockReturnValue(unlock);
        const user = userEvent.setup();
        renderWithProviders(<LockSection user={locked} />);

        // when
        await user.click(screen.getByRole("button", { name: "Unlock user" }));

        // then
        expect(unlock.mutate).toHaveBeenCalledWith({ userId: "user-robin" });
    });

    it("shows the server's reason when the unlock fails", () => {
        // given
        const error = makeApiError({ status: 404, message: "user not found" });
        useUnlockUser.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderWithProviders(<LockSection user={locked} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("user not found");
    });

    it("says the user was unlocked", () => {
        // given
        useUnlockUser.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<LockSection user={open} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("User unlocked.");
    });
});
