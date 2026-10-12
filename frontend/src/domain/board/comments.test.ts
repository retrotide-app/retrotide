import { describe, expect, it } from "vitest";
import { makeCard, makeComment } from "../../test-utils/fixtures";
import { commentCountLabel, commentsOnStack, isEdited } from "./comments";

describe("commentsOnStack", () => {
    it("keeps the comments on any card of the stack, in the order they were written", () => {
        // given
        const stack = [makeCard({ id: "k1" }), makeCard({ id: "k2", parent_card_id: "k1" })];
        const comments = [
            makeComment({ id: "m1", card_id: "k2" }),
            makeComment({ id: "m2", card_id: "k9" }),
            makeComment({ id: "m3", card_id: "k1" }),
        ];

        // when
        const kept = commentsOnStack(comments, stack);

        // then
        expect(kept.map(comment => comment.id)).toEqual(["m1", "m3"]);
    });
});

describe("commentCountLabel", () => {
    it("names the thread by how many comments it holds", () => {
        // then
        expect([0, 1, 4].map(commentCountLabel)).toEqual(["Comment", "1 comment", "4 comments"]);
    });
});

describe("isEdited", () => {
    it("marks a comment changed after it was written, and never one on an anonymous board", () => {
        // then
        expect(isEdited(makeComment({ updated_at: "2026-10-01T09:20:00Z" }))).toBe(true);
        expect(isEdited(makeComment({ updated_at: undefined }))).toBe(false);
        expect(isEdited(makeComment({ created_at: undefined, updated_at: undefined }))).toBe(false);
    });
});
