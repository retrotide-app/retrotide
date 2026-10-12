import { describe, expect, it } from "vitest";
import { makeCardMention, makeCommentMention, makeMention, makePerson } from "../../test-utils/fixtures";
import {
    insertMention,
    keptMentions,
    mentionKey,
    mentionPlaceholder,
    mentionQuery,
    mentionsChanged,
    mentionText,
    suggestPeople,
    titleParts,
} from "./mentions";

const sam = makePerson({ user_id: "u1", display_name: "Sam" });
const samLee = makePerson({ user_id: "u2", display_name: "Sam Lee" });
const ali = makePerson({ user_id: "u3", display_name: "Ali Khan" });

describe("mentionQuery", () => {
    it("finds the name being typed after an @ at the start of a word", () => {
        // when
        const query = mentionQuery("Ask @Sa", 7);

        // then
        expect(query).toEqual({ start: 4, end: 7, text: "Sa" });
    });

    it("opens on a bare @, so the whole list can be shown", () => {
        // when
        const query = mentionQuery("@", 1);

        // then
        expect(query).toEqual({ start: 0, end: 1, text: "" });
    });

    it("keeps a space inside the name, so a full name can be typed", () => {
        // when
        const query = mentionQuery("Ask @Sam L", 10);

        // then
        expect(query?.text).toBe("Sam L");
    });

    it("ignores an @ inside a word, such as an email address", () => {
        // then
        expect(mentionQuery("mail sam@example", 16)).toBeNull();
    });

    it("ignores an @ followed by a space, and text with no @ before the caret", () => {
        // then
        expect(mentionQuery("Ask @ Sam", 9)).toBeNull();
        expect(mentionQuery("Ask Sam", 7)).toBeNull();
    });

    it("reads only up to the caret", () => {
        // when
        const query = mentionQuery("Ask @Sam today", 6);

        // then
        expect(query).toEqual({ start: 4, end: 6, text: "S" });
    });
});

describe("suggestPeople", () => {
    const people = [ali, sam, samLee];

    it("suggests people whose name, or any word of it, starts with what was typed", () => {
        // then
        expect(suggestPeople(people, mentionQuery("@sa", 3))).toEqual([sam, samLee]);
        expect(suggestPeople(people, mentionQuery("@khan", 5))).toEqual([ali]);
    });

    it("suggests everyone on a bare @, and nobody without a query", () => {
        // then
        expect(suggestPeople(people, mentionQuery("@", 1))).toEqual(people);
        expect(suggestPeople(people, null)).toEqual([]);
    });

    it("suggests at most six people", () => {
        // given
        const many = Array.from({ length: 9 }, (_, index) =>
            makePerson({ user_id: `u${index}`, display_name: `Sam ${index}` }),
        );

        // then
        expect(suggestPeople(many, mentionQuery("@sam", 4))).toHaveLength(6);
    });
});

describe("insertMention", () => {
    it("replaces what was typed with the picked name, puts the caret after it and records the person", () => {
        // given
        const query = mentionQuery("Ask @sa about it", 7);

        // when
        const edit = insertMention("Ask @sa about it", query!, samLee, []);

        // then
        expect(edit).toEqual({ title: "Ask @Sam Lee  about it", caret: 13, mentions: [samLee] });
    });

    it("records a person once however often they are picked", () => {
        // given
        const query = mentionQuery("@Sam and @", 10);

        // when
        const edit = insertMention("@Sam and @", query!, sam, [sam]);

        // then
        expect(edit.mentions).toEqual([sam]);
        expect(edit.title).toBe("@Sam and @Sam ");
    });
});

describe("keptMentions", () => {
    it("keeps only the people still named in the title, once each", () => {
        // when
        const kept = keptMentions("Ask @Ali Khan today", [sam, ali, ali]);

        // then
        expect(kept).toEqual([ali]);
    });

    it("drops everyone once their names are deleted", () => {
        // then
        expect(keptMentions("Ask about it", [sam])).toEqual([]);
    });
});

describe("titleParts", () => {
    it("splits a title into plain text and the people it mentions", () => {
        // when
        const parts = titleParts("Ask @Sam and @Ali Khan", [sam, ali]);

        // then
        expect(parts).toEqual([
            { text: "Ask ", mention: null },
            { text: "@Sam", mention: sam },
            { text: " and ", mention: null },
            { text: "@Ali Khan", mention: ali },
        ]);
    });

    it("takes the longest name when one name starts another", () => {
        // when
        const parts = titleParts("@Sam Lee", [sam, samLee]);

        // then
        expect(parts).toEqual([{ text: "@Sam Lee", mention: samLee }]);
    });

    it("leaves a title with no mentions as one plain part", () => {
        // then
        expect(titleParts("Book the room", [])).toEqual([{ text: "Book the room", mention: null }]);
        expect(titleParts("Ask @Sam", [])).toEqual([{ text: "Ask @Sam", mention: null }]);
    });
});

describe("mentionsChanged", () => {
    it("tells a changed set of people from the same people in any order", () => {
        // then
        expect(mentionsChanged([sam, ali], ["u3", "u1"])).toBe(false);
        expect(mentionsChanged([], [])).toBe(false);
        expect(mentionsChanged([sam], [])).toBe(true);
        expect(mentionsChanged([], ["u1"])).toBe(true);
        expect(mentionsChanged([sam], ["u3"])).toBe(true);
    });
});

describe("mentionPlaceholder", () => {
    it("adds the @ hint only while people can be picked", () => {
        // then
        expect(mentionPlaceholder("Write a card, then press Enter", false)).toBe("Write a card, then press Enter");
        expect(mentionPlaceholder("Write a card, then press Enter", true)).toBe(
            "Write a card, then press Enter. Type @ to mention someone",
        );
        expect(mentionPlaceholder("What needs doing?", true)).toBe("What needs doing? Type @ to mention someone");
        expect(mentionPlaceholder(undefined, true)).toBe("Type @ to mention someone");
        expect(mentionPlaceholder(undefined, false)).toBeUndefined();
    });
});

describe("mentionKey and mentionText", () => {
    it("tells each kind of mention apart and reads the text it was made in", () => {
        // given
        const item = makeMention({ action_item_id: "i1", title: "Pair with @Sam" });
        const card = makeCardMention({ card_id: "i1", body: "Thanks @Sam" });
        const comment = makeCommentMention({ comment_id: "i1", body: "Agreed @Sam" });

        // then
        expect([item, card, comment].map(mentionKey)).toEqual(["action_item:i1", "card:i1", "comment:i1"]);
        expect([item, card, comment].map(mentionText)).toEqual(["Pair with @Sam", "Thanks @Sam", "Agreed @Sam"]);
    });
});
