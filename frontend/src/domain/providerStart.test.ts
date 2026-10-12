import { describe, expect, it } from "vitest";
import { makeApiError } from "../test-utils/fixtures";
import { providerStartFailure } from "./providerStart";

describe("providerStartFailure", () => {
    it("says the provider's sign-in is switched off when the start answers 404", () => {
        // given
        const error = makeApiError({ status: 404, message: "not found" });

        // when
        const message = providerStartFailure(error, "sign_in", "Google");

        // then
        expect(message).toBe("Signing in with Google is switched off on this site.");
    });

    it("says linking is switched off when a link start answers 404", () => {
        // given
        const error = makeApiError({ status: 404, message: "not found" });

        // when
        const message = providerStartFailure(error, "link", "Google");

        // then
        expect(message).toBe("Linking Google is switched off on this site.");
    });

    it("names whichever provider could not start", () => {
        // given
        const error = makeApiError({ status: 404, message: "not found" });

        // when
        const message = providerStartFailure(error, "sign_in", "Example ID");

        // then
        expect(message).toBe("Signing in with Example ID is switched off on this site.");
    });

    it("asks for a sign-in method the account already has when the link needs one", () => {
        // given
        const error = makeApiError({
            status: 400,
            message: "confirm the link by signing in with a sign-in method your account already has",
            code: "reauthentication_required",
        });

        // when
        const message = providerStartFailure(error, "link", "Microsoft");

        // then
        expect(message).toBe(
            "Confirm it is you with a sign-in method your account already has before you link Microsoft.",
        );
    });

    it("passes any other refusal through as the server worded it", () => {
        // given
        const error = makeApiError({ status: 429, message: "too many requests, please try again later" });

        // when
        const message = providerStartFailure(error, "sign_in", "Google");

        // then
        expect(message).toBe("too many requests, please try again later");
    });

    it("passes a failure from the browser through as it is worded", () => {
        // given
        const error = new Error("refusing to leave for a javascript: address");

        // when
        const message = providerStartFailure(error, "link", "Google");

        // then
        expect(message).toBe("refusing to leave for a javascript: address");
    });

    it.each([
        ["sign_in", "Signing in with Google failed. Try again."],
        ["link", "Linking your Google account failed. Try again."],
    ] as const)("says the %s start failed when the failure carries no message", (purpose, expected) => {
        // given
        const error = new Error("");

        // when
        const message = providerStartFailure(error, purpose, "Google");

        // then
        expect(message).toBe(expected);
    });
});
