import { afterEach, describe, expect, it } from "vitest";
import { clearLocationToken, leaveForExternalPage, pageOrigin, readLocationToken } from "./location";

afterEach(() => {
    window.history.replaceState(null, "", "/");
});

describe("pageOrigin", () => {
    it("reads the origin the page was served from, without its path", () => {
        // given
        window.history.replaceState(null, "", "/boards/b1?x=1");

        // when
        const origin = pageOrigin();

        // then
        expect(origin).toBe("http://localhost:3000");
    });
});

describe("readLocationToken", () => {
    it("reads the token from the fragment an emailed link carries", () => {
        // given
        window.history.replaceState(null, "", "/verify-email#9f2c0a");

        // when
        const token = readLocationToken();

        // then
        expect(token).toBe("9f2c0a");
    });

    it("decodes a token the link escaped", () => {
        // given
        window.history.replaceState(null, "", "/verify-email#a%20b");

        // when
        const token = readLocationToken();

        // then
        expect(token).toBe("a b");
    });

    it("hands back a fragment that cannot be decoded as it was, for the server to refuse", () => {
        // given
        window.history.replaceState(null, "", "/reset-password#%E0%A4%A");

        // when
        const token = readLocationToken();

        // then
        expect(token).toBe("%E0%A4%A");
    });

    it("finds no token when the address has no fragment", () => {
        // given
        window.history.replaceState(null, "", "/reset-password");

        // when
        const token = readLocationToken();

        // then
        expect(token).toBe("");
    });
});

describe("clearLocationToken", () => {
    it("takes the token out of the address bar and keeps the page and its query", () => {
        // given
        window.history.replaceState(null, "", "/reset-password?from=email#9f2c0a");

        // when
        clearLocationToken();

        // then
        expect(window.location.pathname).toBe("/reset-password");
        expect(window.location.search).toBe("?from=email");
        expect(window.location.hash).toBe("");
    });

    it("leaves an address with no fragment untouched", () => {
        // given
        window.history.replaceState({ kept: true }, "", "/verify-email");

        // when
        clearLocationToken();

        // then
        expect(window.location.pathname).toBe("/verify-email");
        expect(window.history.state).toEqual({ kept: true });
    });
});

describe("leaveForExternalPage", () => {
    it("navigates the whole page to the address it is given", () => {
        // given
        window.history.replaceState(null, "", "/login");
        const destination = `${window.location.origin}/login#google`;

        // when
        leaveForExternalPage(destination);

        // then
        expect(window.location.hash).toBe("#google");
    });

    it.each(["javascript:alert(1)", "data:text/html,hello", "file:///etc/passwd"])(
        "refuses to leave for %s",
        destination => {
            // when
            const attempt = () => leaveForExternalPage(destination);

            // then
            expect(attempt).toThrow("refusing to leave");
        },
    );
});
