import { describe, expect, it } from "vitest";
import { REACTIONS, reactionOption } from "./reactions";

describe("REACTIONS", () => {
    it("lists the fifteen keys the server accepts, in the server's order", () => {
        // when
        const keys = REACTIONS.map(option => option.key);

        // then
        expect(keys).toEqual([
            "thumbs_up",
            "thumbs_down",
            "heart",
            "laugh",
            "wow",
            "sad",
            "angry",
            "party",
            "rocket",
            "fire",
            "clap",
            "thinking",
            "eyes",
            "hundred",
            "star",
        ]);
    });

    it("gives every reaction an emoji and a label of its own", () => {
        // when
        const emoji = new Set(REACTIONS.map(option => option.emoji));
        const labels = new Set(REACTIONS.map(option => option.label));

        // then
        expect(emoji.size).toBe(REACTIONS.length);
        expect(labels.size).toBe(REACTIONS.length);
    });
});

describe("reactionOption", () => {
    it("finds the emoji and label of a key", () => {
        // when
        const heart = reactionOption("heart");

        // then
        expect(heart).toEqual({ key: "heart", emoji: "❤️", label: "Love" });
    });
});
