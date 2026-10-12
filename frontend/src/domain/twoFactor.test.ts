import { describe, expect, it } from "vitest";
import { makeSessionUser } from "../test-utils/fixtures";
import type { TwoFactorChallengeResponse } from "../types/api";
import {
    isTwoFactorChallenge,
    recoveryCodesText,
    TRUSTED_DEVICE_DAYS,
    TWO_FACTOR_CODE_LENGTH,
    TWO_FACTOR_PATH,
} from "./twoFactor";

describe("isTwoFactorChallenge", () => {
    it("recognises a login answer that asks for a second factor", () => {
        // given
        const response: TwoFactorChallengeResponse = { two_factor_required: true };

        // when
        const challenged = isTwoFactorChallenge(response);

        // then
        expect(challenged).toBe(true);
    });

    it("treats a signed-in user as no challenge", () => {
        // given
        const response = makeSessionUser();

        // when
        const challenged = isTwoFactorChallenge(response);

        // then
        expect(challenged).toBe(false);
    });
});

describe("recoveryCodesText", () => {
    it("names the site and lists one code per line, ready to save", () => {
        // given
        const recoveryCodes = ["abcde-fgh23", "jkmnp-qrs45"];

        // when
        const text = recoveryCodesText("RetroTide", recoveryCodes);

        // then
        expect(text).toBe("RetroTide recovery codes\nEach code works once.\n\nabcde-fgh23\njkmnp-qrs45");
    });
});

describe("two-factor constants", () => {
    it("points at the sign-in step page, expects six digit codes and trusts a device for as long as the server does", () => {
        // then
        expect(TWO_FACTOR_PATH).toBe("/two-factor");
        expect(TWO_FACTOR_CODE_LENGTH).toBe(6);
        expect(TRUSTED_DEVICE_DAYS).toBe(30);
    });
});
