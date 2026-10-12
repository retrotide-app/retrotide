import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IdentityListResponse, IdentityProviderResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";
import {
    makeApiError,
    makeIdentity,
    makeIdentityProvider,
    makeMutationState,
    makeProviderSignIn,
    makeQueryState,
    makeSessionUser,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { SignInMethodsSection } from "./SignInMethodsSection";

const { useIdentities, useUnlinkIdentity, useProviderSignIn } = vi.hoisted(() => ({
    useIdentities: vi.fn(),
    useUnlinkIdentity: vi.fn(),
    useProviderSignIn: vi.fn(),
}));

vi.mock("../../hooks/queries/identity", () => ({ useIdentities }));
vi.mock("../../hooks/mutations/identity", () => ({ useUnlinkIdentity }));
vi.mock("../../hooks/useProviderSignIn", () => ({ useProviderSignIn }));

const account = makeSessionUser({ has_password: true, email_verified: true });

const google = makeIdentityProvider();
const example = makeIdentityProvider({ id: "example", name: "Example ID" });

function listing(identities = [makeIdentity()]) {
    return makeQueryState<IdentityListResponse>({ data: { identities } });
}

interface RenderOptions {
    user?: SessionUser;
    providers?: IdentityProviderResponse[];
    route?: string;
}

function renderSection({ user = account, providers = [google], route = "/account" }: RenderOptions = {}) {
    return renderWithProviders(<SignInMethodsSection user={user} />, {
        user,
        siteInfo: { identity_providers: providers },
        route,
        path: "/account",
    });
}

async function openLinkForm(name = "Link Google account") {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name }));

    return user;
}

beforeEach(() => {
    useIdentities.mockReturnValue(listing([]));
    useUnlinkIdentity.mockReturnValue(makeMutationState());
    useProviderSignIn.mockReturnValue(makeProviderSignIn());
});

describe("SignInMethodsSection", () => {
    it("lists the linked sign-in methods", () => {
        // given
        useIdentities.mockReturnValue(listing([makeIdentity({ email: "sam@gmail.com" })]));

        // when
        renderSection();

        // then
        expect(screen.getByRole("region", { name: "Sign-in methods" })).toHaveTextContent("sam@gmail.com");
        expect(screen.getByRole("button", { name: "Remove Google sign-in for sam@gmail.com" })).toBeInTheDocument();
    });

    it("offers to link a provider when the site allows it and none is linked", () => {
        // given
        useIdentities.mockReturnValue(listing([]));

        // when
        renderSection();

        // then
        expect(screen.getByRole("button", { name: "Link Google account" })).toBeInTheDocument();
    });

    it("offers to link each provider the site allows when there are two", () => {
        // given
        useIdentities.mockReturnValue(listing([]));

        // when
        renderSection({ providers: [google, example] });

        // then
        expect(screen.getByRole("button", { name: "Link Google account" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Link Example ID account" })).toBeInTheDocument();
    });

    it("offers to link only the providers not linked yet", () => {
        // given
        useIdentities.mockReturnValue(listing([makeIdentity()]));

        // when
        renderSection({ providers: [google, example] });

        // then
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Link Example ID account" })).toBeInTheDocument();
    });

    it("offers no link once every provider the site allows is linked", () => {
        // given
        const linked = [makeIdentity(), makeIdentity({ provider: "example", provider_name: "Example ID" })];
        useIdentities.mockReturnValue(listing(linked));

        // when
        renderSection({ providers: [google, example] });

        // then
        expect(screen.queryByRole("button", { name: /^Link / })).not.toBeInTheDocument();
    });

    it("offers no link while no provider is switched on", () => {
        // given
        const providers: IdentityProviderResponse[] = [];

        // when
        renderSection({ providers });

        // then
        expect(screen.queryByRole("button", { name: /^Link / })).not.toBeInTheDocument();
    });

    it("offers no second link once the provider's account is linked", () => {
        // given
        useIdentities.mockReturnValue(listing([makeIdentity()]));

        // when
        renderSection();

        // then
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
    });

    it("offers no link until it knows which accounts are linked", () => {
        // given
        useIdentities.mockReturnValue(makeQueryState<IdentityListResponse>({ loading: true }));

        // when
        renderSection();

        // then
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
    });

    it("offers no link to an account with no password and no provider it could confirm with", () => {
        // given
        const user = makeSessionUser({ has_password: false });

        // when
        renderSection({ user });

        // then
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
    });

    it("offers no link to an account with no password whose only provider the site no longer lists", () => {
        // given
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(listing([makeIdentity({ provider: "retired", provider_name: "Retired ID" })]));

        // when
        renderSection({ user, providers: [google] });

        // then
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
    });

    it("offers an account with no password to link another provider beside the one it holds", () => {
        // given
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(listing([makeIdentity()]));

        // when
        renderSection({ user, providers: [google, example] });

        // then
        expect(screen.getByRole("button", { name: "Link Example ID account" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
    });

    it("asks an account with no password to confirm it is them with the provider it holds", async () => {
        // given
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(listing([makeIdentity()]));
        renderSection({ user, providers: [google, example] });

        // when
        await openLinkForm("Link Example ID account");

        // then no password is asked for
        expect(screen.getByRole("dialog", { name: "Link Example ID account" })).toHaveTextContent(
            "Confirm it is you by signing in with Google.",
        );
        expect(screen.queryByLabelText("Current password")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue to Google" })).toBeInTheDocument();
    });

    it("starts the link by confirming with the provider held, and comes back to the account page", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(listing([makeIdentity()]));
        renderSection({ user, providers: [google, example] });
        const clicker = await openLinkForm("Link Example ID account");

        // when
        await clicker.click(screen.getByRole("button", { name: "Continue to Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(example, {
            purpose: "link",
            reauthenticate_with: "google",
            return_to: "/account",
        });
    });

    it("lets an account with no password that holds two providers choose which one confirms the link", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        const third = makeIdentityProvider({ id: "third", name: "Third ID" });
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(
            listing([makeIdentity(), makeIdentity({ provider: "example", provider_name: "Example ID" })]),
        );
        renderSection({ user, providers: [google, example, third] });
        const clicker = await openLinkForm("Link Third ID account");

        // when
        await clicker.selectOptions(screen.getByLabelText("Confirm it is you by signing in with"), "Example ID");
        await clicker.click(screen.getByRole("button", { name: "Continue to Example ID" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(third, {
            purpose: "link",
            reauthenticate_with: "example",
            return_to: "/account",
        });
    });

    it("says why a link confirmed with a provider could not start", async () => {
        // given
        const failure = makeApiError({
            status: 400,
            message: "confirm the link by signing in with a sign-in method your account already has",
            code: "reauthentication_required",
        });
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ error: failure, provider: example }));
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(listing([makeIdentity()]));
        renderSection({ user, providers: [google, example] });

        // when
        await openLinkForm("Link Example ID account");

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "Confirm it is you with a sign-in method your account already has before you link Example ID.",
        );
    });

    it("explains a link refused because the account that confirmed it is not the one linked", () => {
        // given
        const user = makeSessionUser({ has_password: false });
        useIdentities.mockReturnValue(listing([makeIdentity()]));

        // when
        renderSection({
            user,
            providers: [google, example],
            route: "/account?error=reauthentication_failed&provider=google",
        });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "The Google account you signed in with is not the one linked to your account. Try again with that account.",
        );
    });

    it("shows the server's refusal when the account's address turns out unverified, without blaming the password", async () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "verify your email address before linking a sign-in method",
            code: "email_unverified",
        });
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ error: failure, provider: google }));
        renderSection({ user: makeSessionUser({ has_password: true, email_verified: false }) });

        // when
        await openLinkForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "verify your email address before linking a sign-in method",
        );
        expect(screen.getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
        expect(
            screen.queryByText("Verify your email address before you link a sign-in method."),
        ).not.toBeInTheDocument();
    });

    it("asks for the current password before it links the provider", async () => {
        // given
        renderSection();

        // when
        await openLinkForm();

        // then
        expect(screen.getByRole("form", { name: "Link Google account" })).toBeInTheDocument();
        expect(screen.getByLabelText("Current password")).toHaveFocus();
        expect(screen.getByLabelText("Current password")).toHaveAccessibleDescription(
            "Enter your password to link your Google account.",
        );
    });

    it("opens the form of the provider chosen, and only that one, when there are two", async () => {
        // given
        renderSection({ providers: [google, example] });

        // when
        await openLinkForm("Link Example ID account");

        // then
        expect(screen.getByRole("form", { name: "Link Example ID account" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue to Example ID" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Link Google account" })).not.toBeInTheDocument();
        expect(screen.getByLabelText("Current password")).toHaveAccessibleDescription(
            "Enter your password to link your Example ID account.",
        );
    });

    it("starts the link with the password and comes back to the account page", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        renderSection();
        const user = await openLinkForm();

        // when
        await user.type(screen.getByLabelText("Current password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Continue to Google" }));

        // then
        expect(signIn.start).toHaveBeenCalledWith(google, {
            purpose: "link",
            current_password: "harbour-lights",
            return_to: "/account",
        });
    });

    it("starts the link at the provider chosen when there are two", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        renderSection({ providers: [google, example] });
        const user = await openLinkForm("Link Example ID account");

        // when
        await user.type(screen.getByLabelText("Current password"), "harbour-lights");
        await user.click(screen.getByRole("button", { name: "Continue to Example ID" }));

        // then
        expect(signIn.start).toHaveBeenCalledExactlyOnceWith(example, {
            purpose: "link",
            current_password: "harbour-lights",
            return_to: "/account",
        });
    });

    it("asks for the current password before it starts the link, and moves focus to the message", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        renderSection();
        const user = await openLinkForm();

        // when
        await user.click(screen.getByRole("button", { name: "Continue to Google" }));

        // then
        expect(screen.getByLabelText("Current password")).toHaveAccessibleDescription(
            "Enter your password to link your Google account. Enter your current password.",
        );
        expect(screen.getByRole("alert")).toHaveTextContent("Enter your current password.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(signIn.start).not.toHaveBeenCalled();
        expect(signIn.reset).toHaveBeenCalledOnce();
    });

    it("says linking is off when the site switched the provider off after the page loaded", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "not found" });
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ error: failure, provider: google }));
        renderSection();

        // when
        await openLinkForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Linking Google is switched off on this site.");
    });

    it("names the provider being linked when its start fails and there are two", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "not found" });
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ error: failure, provider: example }));
        renderSection({ providers: [google, example] });

        // when
        await openLinkForm("Link Example ID account");

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Linking Example ID is switched off on this site.");
    });

    it("holds the buttons while the page is on its way to the provider", async () => {
        // given
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ pending: true, provider: google }));
        renderSection();

        // when
        await openLinkForm();

        // then
        expect(screen.getByRole("button", { name: "Continue to Google" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    });

    it("ties a wrong password to the field and moves focus to the error", async () => {
        // given
        const failure = makeApiError({
            status: 403,
            message: "the current password is incorrect",
            code: "incorrect_password",
        });
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ error: failure, provider: google }));
        renderSection();

        // when
        await openLinkForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the current password is incorrect");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByLabelText("Current password")).toHaveAttribute("aria-invalid", "true");
    });

    it("shows any other refusal without blaming the password", async () => {
        // given
        const failure = makeApiError({
            status: 503,
            message: "the identity provider could not be discovered",
        });
        useProviderSignIn.mockReturnValue(makeProviderSignIn({ error: failure, provider: google }));
        renderSection();

        // when
        await openLinkForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the identity provider could not be discovered");
        expect(screen.getByLabelText("Current password")).not.toHaveAttribute("aria-invalid");
    });

    it("drops the half-finished link when cancelled", async () => {
        // given
        const signIn = makeProviderSignIn();
        useProviderSignIn.mockReturnValue(signIn);
        renderSection();
        const user = await openLinkForm();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(signIn.reset).toHaveBeenCalledOnce();
        expect(screen.queryByLabelText("Current password")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Link Google account" })).toBeInTheDocument();
    });

    it("offers every unlinked provider again once a link is cancelled", async () => {
        // given
        renderSection({ providers: [google, example] });
        const user = await openLinkForm("Link Example ID account");

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(screen.getByRole("button", { name: "Link Google account" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Link Example ID account" })).toBeInTheDocument();
    });

    it("explains why the provider refused the link it came back from", () => {
        // given
        const route = "/account?error=identity_in_use&provider=google";

        // when
        renderSection({ route });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("That Google account is linked to a different account.");
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("names the provider the refusal came from when there are two", () => {
        // given
        const route = "/account?error=identity_in_use&provider=example";

        // when
        renderSection({ route, providers: [google, example] });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "That Example ID account is linked to a different account.",
        );
    });

    it("says nothing about a refusal from a provider the site does not list", () => {
        // given
        const route = "/account?error=identity_in_use&provider=google";

        // when
        renderSection({ route, providers: [] });

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("uses the link wording for a refusal that differs from sign-in", () => {
        // given
        const route = "/account?error=already_linked&provider=google";

        // when
        renderSection({ route });

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "Your account is already linked to a different Google account. Remove it before you link another.",
        );
    });

    it("says nothing about an error code it does not know", () => {
        // given
        const route = "/account?error=something_else&provider=google";

        // when
        renderSection({ route });

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("says nothing about a refusal that names no provider", () => {
        // given
        const route = "/account?error=identity_in_use";

        // when
        renderSection({ route });

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("clears the earlier refusal once a new link is started", async () => {
        // given
        renderSection({ route: "/account?error=cancelled&provider=google" });

        // when
        await openLinkForm();

        // then
        expect(screen.queryByText("You cancelled linking your Google account.")).not.toBeInTheDocument();
    });
});
