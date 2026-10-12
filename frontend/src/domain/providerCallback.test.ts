import { describe, expect, it } from "vitest";
import { makeIdentityProvider } from "../test-utils/fixtures";
import {
    identityProviderName,
    PROVIDER_CALLBACK_ERROR_PARAM,
    PROVIDER_CALLBACK_PROVIDER_PARAM,
    type ProviderCallbackCode,
    providerCallbackMessage,
} from "./providerCallback";

const EVERY_CODE: readonly ProviderCallbackCode[] = [
    "cancelled",
    "expired",
    "email_unverified",
    "email_not_authoritative",
    "link_required",
    "identity_in_use",
    "already_linked",
    "account_banned",
    "account_unverified",
    "account_not_allowed",
    "reauthentication_failed",
    "failed",
];

const PROVIDERS = [makeIdentityProvider(), makeIdentityProvider({ id: "example", name: "Example ID" })];

describe("PROVIDER_CALLBACK_ERROR_PARAM", () => {
    it("names the query parameter the callback puts its refusal code in", () => {
        // when
        const param = PROVIDER_CALLBACK_ERROR_PARAM;

        // then
        expect(param).toBe("error");
    });
});

describe("PROVIDER_CALLBACK_PROVIDER_PARAM", () => {
    it("names the query parameter the callback puts the provider's id in", () => {
        // when
        const param = PROVIDER_CALLBACK_PROVIDER_PARAM;

        // then
        expect(param).toBe("provider");
    });
});

describe("providerCallbackMessage", () => {
    it.each(EVERY_CODE)("explains %s on the sign in page", code => {
        // when
        const message = providerCallbackMessage(code, "sign_in", "Google");

        // then
        expect(message).toEqual(expect.any(String));
        expect(message).not.toBe("");
    });

    it.each(EVERY_CODE)("explains %s on the account page", code => {
        // when
        const message = providerCallbackMessage(code, "link", "Google");

        // then
        expect(message).toEqual(expect.any(String));
        expect(message).not.toBe("");
    });

    it("tells someone whose provider account this site does not admit that it may not sign in", () => {
        // when
        const message = providerCallbackMessage("account_not_allowed", "sign_in", "Microsoft");

        // then
        expect(message).toBe("That Microsoft account may not sign in to this site.");
    });

    it("words a provider account this site does not admit differently when the user was linking", () => {
        // when
        const message = providerCallbackMessage("account_not_allowed", "link", "Microsoft");

        // then
        expect(message).toBe("That Microsoft account may not be linked on this site.");
    });

    it("tells someone who confirmed a link with the wrong provider account to use the linked one", () => {
        // when
        const message = providerCallbackMessage("reauthentication_failed", "link", "Google");

        // then
        expect(message).toBe(
            "The Google account you signed in with is not the one linked to your account. Try again with that account.",
        );
    });

    it("names the provider the refusal came from", () => {
        // when
        const google = providerCallbackMessage("cancelled", "sign_in", "Google");
        const example = providerCallbackMessage("cancelled", "sign_in", "Example ID");

        // then
        expect(google).toBe("You cancelled signing in with Google.");
        expect(example).toBe("You cancelled signing in with Example ID.");
    });

    it("tells someone the provider cannot vouch for to register with a password and link the provider later", () => {
        // when
        const message = providerCallbackMessage("email_not_authoritative", "sign_in", "Google");

        // then
        expect(message).toContain("Create an account with your email address and a password");
        expect(message).toContain("link Google from your account settings");
    });

    it("tells the owner of a matching account to sign in with their password first", () => {
        // when
        const message = providerCallbackMessage("link_required", "sign_in", "Google");

        // then
        expect(message).toContain("Sign in with your password");
        expect(message).toContain("reset it if you never set one");
    });

    it("words an already linked account differently when the user was linking", () => {
        // when
        const signingIn = providerCallbackMessage("already_linked", "sign_in", "Google");
        const linking = providerCallbackMessage("already_linked", "link", "Google");

        // then
        expect(signingIn).toContain("remove the old Google account");
        expect(linking).toBe(
            "Your account is already linked to a different Google account. Remove it before you link another.",
        );
    });

    it("words a refusal that has nothing to do with the provider the same for every provider", () => {
        // when
        const google = providerCallbackMessage("account_banned", "sign_in", "Google");
        const example = providerCallbackMessage("account_banned", "sign_in", "Example ID");

        // then
        expect(google).toBe("This account is banned.");
        expect(example).toBe(google);
    });

    it("asks the owner of an account whose address is unverified to verify it before signing in", () => {
        // when
        const message = providerCallbackMessage("account_unverified", "sign_in", "Google");

        // then
        expect(message).toBe("Verify your email address before you sign in.");
    });

    it("says nothing when the page carried no code", () => {
        // when
        const message = providerCallbackMessage(null, "sign_in", "Google");

        // then
        expect(message).toBeNull();
    });

    it("says nothing for a code the server never sends", () => {
        // when
        const message = providerCallbackMessage("toString", "sign_in", "Google");

        // then
        expect(message).toBeNull();
    });

    it("says nothing for a code from before the codes were provider-neutral", () => {
        // when
        const message = providerCallbackMessage("google_cancelled", "sign_in", "Google");

        // then
        expect(message).toBeNull();
    });
});

describe("identityProviderName", () => {
    it("names each provider the site lists by its own name", () => {
        // when
        const names = [identityProviderName(PROVIDERS, "google"), identityProviderName(PROVIDERS, "example")];

        // then
        expect(names).toEqual(["Google", "Example ID"]);
    });

    it("names no provider the site does not list, so a crafted link cannot put its own words on the page", () => {
        // when
        const names = [
            identityProviderName([], "google"),
            identityProviderName(PROVIDERS, "Support: email your password to help@evil.example"),
        ];

        // then
        expect(names).toEqual([null, null]);
    });

    it.each([null, ""])("names no provider when the page carried %j", id => {
        // when
        const name = identityProviderName(PROVIDERS, id);

        // then
        expect(name).toBeNull();
    });
});
