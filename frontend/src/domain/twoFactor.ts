import type { TwoFactorChallengeResponse, UserResponse } from "../types/api";

export const TWO_FACTOR_PATH = "/two-factor";

export const TWO_FACTOR_CODE_LENGTH = 6;

export const TRUSTED_DEVICE_DAYS = 30;

export function recoveryCodesText(siteName: string, recoveryCodes: readonly string[]): string {
    return [`${siteName} recovery codes`, "Each code works once.", "", ...recoveryCodes].join("\n");
}

export function isTwoFactorChallenge(
    response: UserResponse | TwoFactorChallengeResponse,
): response is TwoFactorChallengeResponse {
    return "two_factor_required" in response && response.two_factor_required === true;
}
