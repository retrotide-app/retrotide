import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IdentityListResponse, MentionListResponse, TwoFactorStatusResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";
import { makeMutationState, makeProviderSignIn, makeQueryState, makeSessionUser } from "../../test-utils/fixtures";
import { type ProviderOptions, renderWithProviders } from "../../test-utils/render";
import { AccountPage } from "./AccountPage";

const mocks = vi.hoisted(() => ({
    useUpdateProfile: vi.fn(),
    useSetEmail: vi.fn(),
    useChangePassword: vi.fn(),
    useForgotPassword: vi.fn(),
    useSignOutOtherDevices: vi.fn(),
    useDeleteAccount: vi.fn(),
    useIdentities: vi.fn(),
    useUnlinkIdentity: vi.fn(),
    useProviderSignIn: vi.fn(),
    useMentions: vi.fn(),
    useTwoFactorStatus: vi.fn(),
}));

vi.mock("../../hooks/mutations/auth", () => ({
    useUpdateProfile: mocks.useUpdateProfile,
    useSetEmail: mocks.useSetEmail,
    useChangePassword: mocks.useChangePassword,
    useForgotPassword: mocks.useForgotPassword,
    useSignOutOtherDevices: mocks.useSignOutOtherDevices,
    useDeleteAccount: mocks.useDeleteAccount,
}));
vi.mock("../../hooks/queries/identity", () => ({ useIdentities: mocks.useIdentities }));
vi.mock("../../hooks/mutations/identity", () => ({ useUnlinkIdentity: mocks.useUnlinkIdentity }));
vi.mock("../../hooks/useProviderSignIn", () => ({ useProviderSignIn: mocks.useProviderSignIn }));
vi.mock("../../hooks/queries/actionItems", () => ({ useMentions: mocks.useMentions }));
vi.mock("../../hooks/queries/twoFactor", () => ({ useTwoFactorStatus: mocks.useTwoFactorStatus }));

const account = makeSessionUser({ email: "sam@example.test", has_password: true });

const SECTIONS = [
    "Profile",
    "Mentions",
    "Email address",
    "Password",
    "Two-factor authentication",
    "Sign-in methods",
    "Other devices",
    "Your data",
    "Delete account",
];

function succeeding() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
            options?.onSuccess?.();
        }),
    });
}

function emailSection() {
    return within(screen.getByRole("region", { name: "Email address" }));
}

async function requestEmailChange(address: string) {
    const user = userEvent.setup();
    await user.click(emailSection().getByRole("button", { name: "Change email address" }));
    const dialog = within(screen.getByRole("dialog", { name: "Change email address" }));
    await user.type(dialog.getByLabelText("New email address"), address);
    await user.type(dialog.getByLabelText("Current password"), "harbour-lights");
    await user.click(dialog.getByRole("button", { name: "Send link" }));

    return user;
}

beforeEach(() => {
    mocks.useUpdateProfile.mockReturnValue(makeMutationState());
    mocks.useSetEmail.mockReturnValue(succeeding());
    mocks.useChangePassword.mockReturnValue(makeMutationState());
    mocks.useForgotPassword.mockReturnValue(makeMutationState());
    mocks.useSignOutOtherDevices.mockReturnValue(makeMutationState());
    mocks.useDeleteAccount.mockReturnValue(makeMutationState());
    mocks.useIdentities.mockReturnValue(makeQueryState<IdentityListResponse>({ data: { identities: [] } }));
    mocks.useUnlinkIdentity.mockReturnValue(makeMutationState());
    mocks.useProviderSignIn.mockReturnValue(makeProviderSignIn());
    mocks.useMentions.mockReturnValue(makeQueryState<MentionListResponse>({ data: { mentions: [] } }));
    mocks.useTwoFactorStatus.mockReturnValue(makeQueryState<TwoFactorStatusResponse>({ data: { enabled: false } }));
});

describe("AccountPage", () => {
    it("heads the page and draws every account section", () => {
        // given
        const user = account;

        // when
        renderWithProviders(<AccountPage />, { user });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Account" })).toBeInTheDocument();
        for (const section of SECTIONS) {
            expect(screen.getByRole("region", { name: section })).toBeInTheDocument();
        }
    });

    it("names the browser tab after the page", () => {
        // given
        const user = account;

        // when
        renderWithProviders(<AccountPage />, { user, siteInfo: { site_name: "RetroTide" } });

        // then
        expect(document.title).toContain("Account");
        expect(document.title).toContain("RetroTide");
    });

    it("shows the new address as pending once its link is sent", async () => {
        // given
        renderWithProviders(<AccountPage />, { user: account });

        // when
        await requestEmailChange("sam@new.example.test");

        // then
        expect(emailSection().getByRole("status")).toHaveTextContent("We sent a link to sam@new.example.test.");
        expect(emailSection().getByText(/Your email address is/)).toHaveTextContent(
            "Your email address is sam@example.test.",
        );
    });

    it("stops showing the pending address once its link is used", async () => {
        // given
        const options: ProviderOptions & { user: SessionUser } = { user: account };
        const { rerender } = renderWithProviders(<AccountPage />, options);
        await requestEmailChange("sam@new.example.test");

        // when
        options.user = makeSessionUser({ email: "Sam@New.example.test", has_password: true });
        rerender(<AccountPage />);

        // then
        expect(emailSection().queryByRole("status")).not.toBeInTheDocument();
        expect(emailSection().getByText(/Your email address is/)).toHaveTextContent(
            "Your email address is Sam@New.example.test.",
        );
    });

    it("drops the pending address once the other devices are signed out, which cancels it", async () => {
        // given
        mocks.useSignOutOtherDevices.mockReturnValue(succeeding());
        renderWithProviders(<AccountPage />, { user: account });
        const user = await requestEmailChange("sam@new.example.test");

        // when
        await user.click(screen.getByRole("button", { name: "Sign out other devices" }));

        // then
        expect(emailSection().queryByRole("status")).not.toBeInTheDocument();
    });

    it("drops the pending address once the password changes, which cancels it", async () => {
        // given
        mocks.useChangePassword.mockReturnValue(succeeding());
        renderWithProviders(<AccountPage />, { user: account });
        const user = await requestEmailChange("sam@new.example.test");
        await user.click(
            within(screen.getByRole("region", { name: "Password" })).getByRole("button", { name: "Change password" }),
        );
        const password = within(screen.getByRole("dialog", { name: "Change password" }));

        // when
        await user.type(password.getByLabelText("Current password"), "harbour-lights");
        await user.type(password.getByLabelText("New password"), "lantern-tide-42");
        await user.click(password.getByRole("button", { name: "Save new password" }));

        // then
        expect(emailSection().queryByRole("status")).not.toBeInTheDocument();
    });

    it("steers an account with no password to set one before anything that needs it", () => {
        // given
        const user = makeSessionUser({ has_password: false });

        // when
        renderWithProviders(<AccountPage />, { user });

        // then
        expect(screen.getByRole("button", { name: "Set a password" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Change password" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Change email address" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Delete account" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Sign out other devices" })).toBeInTheDocument();
    });

    it("draws nothing once nobody is signed in", () => {
        // given
        const user = null;

        // when
        const { container } = renderWithProviders(<AccountPage />, { user });

        // then
        expect(container).toBeEmptyDOMElement();
    });
});
