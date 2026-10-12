import { describe, expect, it } from "vitest";
import { accountInitials, DISPLAY_NAME_MAX_LENGTH, initialsOf } from "./displayName";

describe("DISPLAY_NAME_MAX_LENGTH", () => {
    it("matches the length the server cuts a display name to", () => {
        // given
        const serverLimit = 40;

        // when
        const limit = DISPLAY_NAME_MAX_LENGTH;

        // then
        expect(limit).toBe(serverLimit);
    });
});

describe("initialsOf", () => {
    it("takes the first letter of the first two words", () => {
        // when
        const initials = initialsOf("sam  de la cruz");

        // then
        expect(initials).toBe("SD");
    });

    it("takes one letter from a single word", () => {
        // when
        const initials = initialsOf("Robin");

        // then
        expect(initials).toBe("R");
    });

    it("keeps a letter outside the basic plane whole", () => {
        // when
        const initials = initialsOf("𝒜da Byron");

        // then
        expect(initials).toBe("𝒜B");
    });

    it("gives nothing for a blank name", () => {
        // when
        const initials = initialsOf("   ");

        // then
        expect(initials).toBe("");
    });
});

describe("accountInitials", () => {
    it("uses the display name when there is one", () => {
        // when
        const initials = accountInitials("Sam de la Cruz", "robin@example.test");

        // then
        expect(initials).toBe("SD");
    });

    it("falls back to the first letter of the email address when the name is blank", () => {
        // when
        const initials = accountInitials("  ", "robin@example.test");

        // then
        expect(initials).toBe("R");
    });

    it("keeps a first letter outside the basic plane whole", () => {
        // when
        const initials = accountInitials("", "𝒜da@example.test");

        // then
        expect(initials).toBe("𝒜");
    });
});
