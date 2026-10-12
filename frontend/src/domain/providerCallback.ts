import type { IdentityProviderResponse, IdentityPurpose } from "../types/api";

export type ProviderCallbackCode =
    | "cancelled"
    | "expired"
    | "email_unverified"
    | "email_not_authoritative"
    | "link_required"
    | "identity_in_use"
    | "already_linked"
    | "account_banned"
    | "account_unverified"
    | "account_not_allowed"
    | "reauthentication_failed"
    | "failed";

type ProviderMessage = (provider: string) => string;

export const PROVIDER_CALLBACK_ERROR_PARAM = "error";

export const PROVIDER_CALLBACK_PROVIDER_PARAM = "provider";

const SIGN_IN_MESSAGES: Readonly<Record<ProviderCallbackCode, ProviderMessage>> = {
    cancelled: provider => `You cancelled signing in with ${provider}.`,
    expired: provider => `Signing in with ${provider} took too long. Try again.`,
    email_unverified: provider => `${provider} has not verified the email address on that ${provider} account.`,
    email_not_authoritative: provider =>
        `${provider} cannot confirm that this email address is yours. Create an account with your email address and a password, verify the address, then link ${provider} from your account settings.`,
    link_required: provider =>
        `An account already uses this email address. Sign in with your password, or reset it if you never set one, then link ${provider} from your account settings.`,
    identity_in_use: provider => `That ${provider} account is linked to a different account.`,
    already_linked: provider =>
        `Your account is linked to a different ${provider} account. Sign in with your password, or reset it if you never set one, then remove the old ${provider} account from your account settings.`,
    account_banned: () => "This account is banned.",
    account_unverified: () => "Verify your email address before you sign in.",
    account_not_allowed: provider => `That ${provider} account may not sign in to this site.`,
    reauthentication_failed: provider =>
        `The ${provider} account you signed in with is not the one linked to your account. Try again with that account.`,
    failed: provider => `Signing in with ${provider} failed. Try again.`,
};

const LINK_MESSAGES: Readonly<Record<ProviderCallbackCode, ProviderMessage>> = {
    ...SIGN_IN_MESSAGES,
    cancelled: provider => `You cancelled linking your ${provider} account.`,
    expired: provider =>
        `Linking your ${provider} account took too long, or you signed out before it finished. Try again.`,
    link_required: provider => `Your account changed while ${provider} was checking it. Try again.`,
    already_linked: provider =>
        `Your account is already linked to a different ${provider} account. Remove it before you link another.`,
    account_not_allowed: provider => `That ${provider} account may not be linked on this site.`,
    failed: provider => `Linking your ${provider} account failed. Try again.`,
};

function isProviderCallbackCode(code: string): code is ProviderCallbackCode {
    return Object.hasOwn(SIGN_IN_MESSAGES, code);
}

export function identityProviderName(providers: readonly IdentityProviderResponse[], id: string | null): string | null {
    const listed = providers.find(provider => provider.id === id);

    return listed?.name ?? null;
}

export function providerCallbackMessage(
    code: string | null,
    purpose: IdentityPurpose,
    providerName: string,
): string | null {
    if (code === null || !isProviderCallbackCode(code)) {
        return null;
    }

    const messages = purpose === "link" ? LINK_MESSAGES : SIGN_IN_MESSAGES;

    return messages[code](providerName);
}
