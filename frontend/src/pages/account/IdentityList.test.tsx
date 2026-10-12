import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IdentityListResponse } from "../../types/api";
import { makeApiError, makeIdentity, makeMutationState, makeQueryState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { formatExactDateTime } from "../../utils/time";
import { IdentityList } from "./IdentityList";

const { useIdentities, useUnlinkIdentity } = vi.hoisted(() => ({
    useIdentities: vi.fn(),
    useUnlinkIdentity: vi.fn(),
}));

vi.mock("../../hooks/queries/identity", () => ({ useIdentities }));
vi.mock("../../hooks/mutations/identity", () => ({ useUnlinkIdentity }));

const google = makeIdentity({ email: "sam@gmail.com", created_at: "2026-02-10T14:00:00Z" });
const example = makeIdentity({
    provider: "example",
    provider_name: "Example ID",
    email: "sam@example.test",
    created_at: "2026-03-01T08:15:00Z",
});

function listing(identities = [google]) {
    return makeQueryState<IdentityListResponse>({ data: { identities } });
}

function settling() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

beforeEach(() => {
    useIdentities.mockReturnValue(listing());
    useUnlinkIdentity.mockReturnValue(makeMutationState());
});

describe("IdentityList", () => {
    it("says the sign-in methods are loading", () => {
        // given
        useIdentities.mockReturnValue(makeQueryState<IdentityListResponse>({ loading: true }));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByText("Loading sign-in methods...")).toBeInTheDocument();
    });

    it("shows why the sign-in methods could not load, and loads them again when asked", async () => {
        // given
        const failed = makeQueryState<IdentityListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the sign-in methods" }),
        });
        useIdentities.mockReturnValue(failed);
        const user = userEvent.setup();
        renderWithProviders(<IdentityList hasPassword />);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the sign-in methods");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("says when no other account is linked", () => {
        // given
        useIdentities.mockReturnValue(listing([]));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByText("No other accounts are linked.")).toBeInTheDocument();
        expect(screen.queryByRole("list", { name: "Linked sign-in methods" })).not.toBeInTheDocument();
    });

    it("lists each linked account with its address and when it was linked", () => {
        // given
        useIdentities.mockReturnValue(listing([google]));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        const item = within(screen.getByRole("list", { name: "Linked sign-in methods" })).getByRole("listitem");
        expect(item).toHaveTextContent("Google");
        expect(item).toHaveTextContent("sam@gmail.com");
        expect(item).toHaveTextContent(`Linked ${formatExactDateTime("2026-02-10T14:00:00Z")}`);
    });

    it("names each linked account by its provider's name when two are linked", () => {
        // given
        useIdentities.mockReturnValue(listing([google, example]));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        const items = within(screen.getByRole("list", { name: "Linked sign-in methods" })).getAllByRole("listitem");
        expect(items).toHaveLength(2);
        expect(items[0]).toHaveTextContent("Google");
        expect(items[0]).toHaveTextContent("sam@gmail.com");
        expect(items[1]).toHaveTextContent("Example ID");
        expect(items[1]).toHaveTextContent("sam@example.test");
        expect(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" })).toBeInTheDocument();
        expect(
            screen.getByRole("button", { name: "Remove Example ID sign-in for sam@example.test" }),
        ).toBeInTheDocument();
    });

    it("names a linked account whose provider is gone by the name the server gives it", () => {
        // given
        const orphan = makeIdentity({ provider: "retired", provider_name: "retired", email: "sam@retired.test" });
        useIdentities.mockReturnValue(listing([orphan]));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByRole("button", { name: "Remove retired sign-in for sam@retired.test" })).toBeInTheDocument();
    });

    it("asks before removing a sign-in method", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<IdentityList hasPassword />);

        // when
        await user.click(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "Remove Google sign-in?" });
        expect(dialog).toHaveTextContent("You will no longer be able to sign in with sam@gmail.com.");
    });

    it("removes the sign-in method once confirmed, then closes the question", async () => {
        // given
        const unlink = settling();
        useUnlinkIdentity.mockReturnValue(unlink);
        const user = userEvent.setup();
        renderWithProviders(<IdentityList hasPassword />);
        await user.click(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" }));

        // when
        await user.click(screen.getByRole("button", { name: "Remove" }));

        // then
        expect(unlink.mutate).toHaveBeenCalledWith(
            google,
            expect.objectContaining({ onSettled: expect.any(Function) }),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("asks about and removes the account that was chosen when two are linked", async () => {
        // given
        const unlink = settling();
        useUnlinkIdentity.mockReturnValue(unlink);
        useIdentities.mockReturnValue(listing([google, example]));
        const user = userEvent.setup();
        renderWithProviders(<IdentityList hasPassword />);
        await user.click(screen.getByRole("button", { name: "Remove Example ID sign-in for sam@example.test" }));
        const dialog = screen.getByRole("dialog", { name: "Remove Example ID sign-in?" });

        // when
        await user.click(within(dialog).getByRole("button", { name: "Remove" }));

        // then
        expect(unlink.mutate).toHaveBeenCalledExactlyOnceWith(
            example,
            expect.objectContaining({ onSettled: expect.any(Function) }),
        );
    });

    it("keeps the sign-in method when the question is cancelled", async () => {
        // given
        const unlink = makeMutationState();
        useUnlinkIdentity.mockReturnValue(unlink);
        const user = userEvent.setup();
        renderWithProviders(<IdentityList hasPassword />);
        await user.click(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" }));

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(unlink.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("clears an earlier refusal before asking again", async () => {
        // given
        const unlink = makeMutationState();
        useUnlinkIdentity.mockReturnValue(unlink);
        const user = userEvent.setup();
        renderWithProviders(<IdentityList hasPassword />);

        // when
        await user.click(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" }));

        // then
        expect(unlink.reset).toHaveBeenCalledOnce();
    });

    it("says the sign-in method was removed", () => {
        // given
        useUnlinkIdentity.mockReturnValue(makeMutationState({ isSuccess: true, variables: google }));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Google sign-in removed.");
    });

    it("names the removed account's provider even once it has left the list", () => {
        // given
        useIdentities.mockReturnValue(listing([google]));
        useUnlinkIdentity.mockReturnValue(makeMutationState({ isSuccess: true, variables: example }));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Example ID sign-in removed.");
    });

    it("explains that the last way to sign in cannot be removed", () => {
        // given
        const failure = makeApiError({
            status: 409,
            message: "that is the account's last way to sign in",
            code: "last_sign_in_method",
        });
        useUnlinkIdentity.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This is your only way to sign in. Set a password before you remove it.",
        );
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("shows the server's reason for any other refusal", () => {
        // given
        const failure = makeApiError({ status: 404, message: "the account has no identity from that provider" });
        useUnlinkIdentity.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<IdentityList hasPassword />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the account has no identity from that provider");
    });

    it("holds the question while the sign-in method is being removed", async () => {
        // given
        useUnlinkIdentity.mockReturnValue(makeMutationState());
        const user = userEvent.setup();
        const { rerender } = renderWithProviders(<IdentityList hasPassword />);
        await user.click(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" }));

        // when
        useUnlinkIdentity.mockReturnValue(makeMutationState({ isPending: true }));
        rerender(<IdentityList hasPassword />);

        // then
        expect(screen.getByRole("button", { name: "Remove" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    });

    it("offers no removal of the only way into an account with no password, and says why", () => {
        // given
        const hasPassword = false;

        // when
        renderWithProviders(<IdentityList hasPassword={hasPassword} />);

        // then
        expect(screen.queryByRole("button", { name: /Remove/ })).not.toBeInTheDocument();
        expect(
            screen.getByText("This is your only way to sign in. Set a password before you remove it."),
        ).toBeInTheDocument();
    });
});
