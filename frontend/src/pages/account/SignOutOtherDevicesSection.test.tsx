import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IdentityListResponse } from "../../types/api";
import {
    makeApiError,
    makeIdentity,
    makeMutationState,
    makeQueryState,
    makeSessionUser,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { SignOutOtherDevicesSection } from "./SignOutOtherDevicesSection";

const { useSignOutOtherDevices, useIdentities, useUnlinkIdentity } = vi.hoisted(() => ({
    useSignOutOtherDevices: vi.fn(),
    useIdentities: vi.fn(),
    useUnlinkIdentity: vi.fn(),
}));

vi.mock("../../hooks/mutations/auth", () => ({ useSignOutOtherDevices }));
vi.mock("../../hooks/queries/identity", () => ({ useIdentities }));
vi.mock("../../hooks/mutations/identity", () => ({ useUnlinkIdentity }));

const account = makeSessionUser({ has_password: true });

beforeEach(() => {
    useSignOutOtherDevices.mockReturnValue(makeMutationState());
    useIdentities.mockReturnValue(
        makeQueryState<IdentityListResponse>({ data: { identities: [makeIdentity({ email: "sam@gmail.com" })] } }),
    );
    useUnlinkIdentity.mockReturnValue(makeMutationState());
});

describe("SignOutOtherDevicesSection", () => {
    it("says that it also cancels an unconfirmed change of address", () => {
        // given
        const user = account;

        // when
        renderWithProviders(<SignOutOtherDevicesSection user={user} onSignedOut={vi.fn()} />, { user });

        // then
        expect(screen.getByRole("region", { name: "Other devices" })).toHaveTextContent(
            "Sign out on every other browser and device. This also cancels a change of email address you have not confirmed.",
        );
    });

    it("signs out the other devices and reports it", async () => {
        // given
        const signOut = makeMutationState({
            mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
                options?.onSuccess?.();
            }),
        });
        useSignOutOtherDevices.mockReturnValue(signOut);
        const onSignedOut = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<SignOutOtherDevicesSection user={account} onSignedOut={onSignedOut} />, {
            user: account,
        });

        // when
        await user.click(screen.getByRole("button", { name: "Sign out other devices" }));

        // then
        expect(signOut.mutate).toHaveBeenCalledWith(undefined, expect.objectContaining({ onSuccess: onSignedOut }));
        expect(onSignedOut).toHaveBeenCalledOnce();
    });

    it("lists no sign-in methods before the other devices are signed out", () => {
        // given
        useSignOutOtherDevices.mockReturnValue(makeMutationState());

        // when
        renderWithProviders(<SignOutOtherDevicesSection user={account} onSignedOut={vi.fn()} />, { user: account });

        // then
        expect(screen.queryByRole("list", { name: "Linked sign-in methods" })).not.toBeInTheDocument();
    });

    it("lists the linked sign-in methods with a Remove action once the other devices are signed out", () => {
        // given
        useSignOutOtherDevices.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<SignOutOtherDevicesSection user={account} onSignedOut={vi.fn()} />, { user: account });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("You are signed out on every other device.");
        expect(screen.getByRole("heading", { level: 3, name: "Check your sign-in methods" })).toBeInTheDocument();
        expect(screen.getByText("Remove any you do not recognise.")).toBeInTheDocument();
        const list = screen.getByRole("list", { name: "Linked sign-in methods" });
        expect(within(list).getByText("sam@gmail.com")).toBeInTheDocument();
        expect(
            within(list).getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" }),
        ).toBeInTheDocument();
    });

    it("keeps the only way into an account with no password once the other devices are signed out", () => {
        // given
        const user = makeSessionUser({ has_password: false });
        useSignOutOtherDevices.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<SignOutOtherDevicesSection user={user} onSignedOut={vi.fn()} />, { user });

        // then
        expect(screen.queryByRole("button", { name: /Remove/ })).not.toBeInTheDocument();
    });

    it("holds the button while the other devices are being signed out", () => {
        // given
        useSignOutOtherDevices.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<SignOutOtherDevicesSection user={account} onSignedOut={vi.fn()} />, { user: account });

        // then
        expect(screen.getByRole("button", { name: "Sign out other devices" })).toBeDisabled();
    });

    it("shows the server's reason when the other devices could not be signed out and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to sign out the other devices" });
        useSignOutOtherDevices.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<SignOutOtherDevicesSection user={account} onSignedOut={vi.fn()} />, { user: account });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to sign out the other devices");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });
});
