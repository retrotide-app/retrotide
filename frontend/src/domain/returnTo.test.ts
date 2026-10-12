import { describe, expect, it } from "vitest";
import { isSafeReturnPath, returnPathFrom, returnState } from "./returnTo";

describe("isSafeReturnPath", () => {
    const safe = ["/", "/account", "/admin/users?search=sam", "/b/abc123", "/o/acme/settings"];
    const unsafe = [
        "",
        "account",
        "//evil.example",
        "/\\evil.example",
        "https://evil.example",
        "javascript:alert(1)",
        "/account\tx",
        "/account\nx",
        "/api",
        "/api/v1/auth/session",
        "/API/v1/auth/session",
        "/api?x=1",
    ];

    it.each(safe)("accepts %s", path => {
        // when
        const accepted = isSafeReturnPath(path);

        // then
        expect(accepted).toBe(true);
    });

    it.each(unsafe)("refuses %j", path => {
        // when
        const accepted = isSafeReturnPath(path);

        // then
        expect(accepted).toBe(false);
    });

    it("accepts a path that merely starts with the letters api", () => {
        // given
        const path = "/apiary";

        // when
        const accepted = isSafeReturnPath(path);

        // then
        expect(accepted).toBe(true);
    });
});

describe("returnState", () => {
    it("joins the path and the query into the place to come back to", () => {
        // given
        const pathname = "/admin/users";
        const search = "?search=sam";

        // when
        const state = returnState(pathname, search);

        // then
        expect(state).toEqual({ from: "/admin/users?search=sam" });
    });
});

describe("returnPathFrom", () => {
    it("reads the path back out of the state a guard left behind", () => {
        // given
        const state = returnState("/account", "");

        // when
        const path = returnPathFrom(state);

        // then
        expect(path).toBe("/account");
    });

    it("refuses a path that would leave the site", () => {
        // given
        const state = { from: "//evil.example" };

        // when
        const path = returnPathFrom(state);

        // then
        expect(path).toBeNull();
    });

    it.each([null, undefined, "/account", 42, {}, { from: 42 }])("finds nothing in %j", state => {
        // when
        const path = returnPathFrom(state);

        // then
        expect(path).toBeNull();
    });
});
