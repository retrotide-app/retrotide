import { describe, expect, it } from "vitest";
import { errorCode, errorMessage, errorStatus } from "./errorMessage";

class ApiError extends Error {
    status: number;
    code: string | null;
    body: unknown;

    constructor(status: number, message: string, body: unknown, code: string | null = null) {
        super(message);
        this.status = status;
        this.code = code;
        this.body = body;
    }
}

describe("errorMessage", () => {
    it("returns the message of a plain Error", () => {
        // given
        const thrown = new Error("the board could not be reached");

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("the board could not be reached");
    });

    it("returns the server's message from an ApiError, because api/client already unwrapped the body", () => {
        // given
        const body = { error: "that board name is already taken" };
        const thrown = new ApiError(409, body.error, body);

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("that board name is already taken");
    });

    it("does not leak the status or the raw body of an ApiError into the message", () => {
        // given
        const thrown = new ApiError(500, "API error: 500", { error: "API error: 500", trace: "abc" });

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("API error: 500");
        expect(result).not.toContain("trace");
    });

    it("falls back when a server body carried an empty error string, which api/client passes through", () => {
        // given
        const thrown = new ApiError(400, "", { error: "" });

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("returns a bare thrown string, because a thrown string is already the message", () => {
        // given
        const thrown = "Failed to fetch";

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to fetch");
    });

    it("falls back for a blank thrown string", () => {
        // given
        const thrown = "   ";

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("falls back for null", () => {
        // given
        const thrown = null;

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("falls back for undefined", () => {
        // given
        const thrown = undefined;

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("falls back for an object with no message rather than rendering [object Object]", () => {
        // given
        const thrown = { status: 500, body: null };

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("reads a message off an error-like object that fails instanceof, such as one crossing a realm", () => {
        // given
        const thrown = { name: "TypeError", message: "Load failed", stack: "TypeError: Load failed" };

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Load failed");
    });

    it("falls back when message is present but is not a string", () => {
        // given
        const thrown = { message: 404 };

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("falls back for a thrown primitive that is neither a string nor an object", () => {
        // given
        const thrown = 404;

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("Failed to save the card");
    });

    it("trims the message so padding never reaches the user", () => {
        // given
        const thrown = new Error("  the retrospective has already ended\n");

        // when
        const result = errorMessage(thrown, "Failed to save the card");

        // then
        expect(result).toBe("the retrospective has already ended");
    });

    it("returns the fallback exactly as given, without decorating it", () => {
        // given
        const thrown = new Error("   ");

        // when
        const result = errorMessage(thrown, "Could not add the action item");

        // then
        expect(result).toBe("Could not add the action item");
    });
});

describe("errorCode", () => {
    it("returns the code the server attached to an ApiError", () => {
        // given
        const thrown = new ApiError(403, "wrong password", { error: "wrong password" }, "incorrect_password");

        // when
        const code = errorCode(thrown);

        // then
        expect(code).toBe("incorrect_password");
    });

    it("returns null for an ApiError the server gave no code", () => {
        // given
        const thrown = new ApiError(500, "internal error", { error: "internal error" });

        // when
        const code = errorCode(thrown);

        // then
        expect(code).toBeNull();
    });

    it("returns null for an empty code", () => {
        // given
        const thrown = { code: "" };

        // when
        const code = errorCode(thrown);

        // then
        expect(code).toBeNull();
    });

    it("returns null for a plain Error, a string and nothing at all", () => {
        // given
        const thrown = [new Error("boom"), "boom", null, undefined, 42];

        // when
        const codes = thrown.map(errorCode);

        // then
        expect(codes).toEqual([null, null, null, null, null]);
    });

    it("returns null when the code is not a string", () => {
        // given
        const thrown = { code: 403 };

        // when
        const code = errorCode(thrown);

        // then
        expect(code).toBeNull();
    });
});

describe("errorStatus", () => {
    it("returns the HTTP status of an ApiError", () => {
        // given
        const thrown = new ApiError(429, "too many requests, please try again later", {});

        // when
        const status = errorStatus(thrown);

        // then
        expect(status).toBe(429);
    });

    it("returns null for a failure that never reached the server", () => {
        // given
        const thrown = new TypeError("Failed to fetch");

        // when
        const status = errorStatus(thrown);

        // then
        expect(status).toBeNull();
    });

    it("returns null when the status is not a number", () => {
        // given
        const thrown = { status: "503" };

        // when
        const status = errorStatus(thrown);

        // then
        expect(status).toBeNull();
    });
});
