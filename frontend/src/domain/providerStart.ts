import type { IdentityPurpose } from "../types/api";
import { errorCode, errorMessage, errorStatus } from "../utils/errorMessage";

type ProviderMessage = (provider: string) => string;

const REAUTHENTICATION_REQUIRED = "reauthentication_required";

const SWITCHED_OFF: Readonly<Record<IdentityPurpose, ProviderMessage>> = {
    sign_in: provider => `Signing in with ${provider} is switched off on this site.`,
    link: provider => `Linking ${provider} is switched off on this site.`,
};

const FAILED: Readonly<Record<IdentityPurpose, ProviderMessage>> = {
    sign_in: provider => `Signing in with ${provider} failed. Try again.`,
    link: provider => `Linking your ${provider} account failed. Try again.`,
};

export function providerStartFailure(error: Error, purpose: IdentityPurpose, providerName: string): string {
    if (errorStatus(error) === 404) {
        return SWITCHED_OFF[purpose](providerName);
    }

    if (errorCode(error) === REAUTHENTICATION_REQUIRED) {
        return `Confirm it is you with a sign-in method your account already has before you link ${providerName}.`;
    }

    return errorMessage(error, FAILED[purpose](providerName));
}
