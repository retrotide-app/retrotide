import { describe, expect, it } from "vitest";
import {
    makeActionItem,
    makeActionItemListItem,
    makeBoardSnapshot,
    makeCard,
    makePerson,
    makeTeam,
} from "../../test-utils/fixtures";
import {
    actionCountLabel,
    assigneeChoices,
    assigneeQuery,
    cardActionCount,
    completionLabel,
    dueOnFrom,
    formatDueDate,
    nextStatus,
    pickedPerson,
    revealedCardId,
    sourceCard,
    statusFilterOptions,
    statusQuery,
    teamChoices,
    teamQuery,
    titleFromCard,
} from "./actionItems";

describe("formatDueDate", () => {
    it("writes a due date out in full on the day it names, whatever the viewer's time zone", () => {
        // then
        expect(formatDueDate("2026-10-20")).toBe("20 Oct 2026");
        expect(formatDueDate("2026-01-01")).toBe("1 Jan 2026");
    });

    it("writes nothing for an item with no due date", () => {
        // then
        expect(formatDueDate(null)).toBe("");
    });
});

describe("titleFromCard", () => {
    it("folds a card's lines into one title line", () => {
        // then
        expect(titleFromCard("  Standups\n ran   long ")).toBe("Standups ran long");
    });

    it("cuts a long card down to the longest title an action item takes", () => {
        // when
        const title = titleFromCard("a".repeat(600));

        // then
        expect(title).toHaveLength(500);
    });
});

describe("dueOnFrom", () => {
    it("reads an empty date field as no due date", () => {
        // then
        expect(dueOnFrom("")).toBeNull();
        expect(dueOnFrom("  ")).toBeNull();
        expect(dueOnFrom("2026-10-20")).toBe("2026-10-20");
    });
});

describe("cardActionCount", () => {
    it("counts the action items made from a card and the cards merged into it", () => {
        // given
        const head = makeCard({ id: "k1" });
        const merged = makeCard({ id: "k2", parent_card_id: "k1" });
        const snapshot = makeBoardSnapshot({
            action_items: [
                makeActionItem({ id: "i1", source_card_id: "k1" }),
                makeActionItem({ id: "i2", source_card_id: "k1" }),
                makeActionItem({ id: "i3", source_card_id: "k2" }),
                makeActionItem({ id: "i4", source_card_id: "k9" }),
                makeActionItem({ id: "i5", source_card_id: null }),
            ],
        });

        // when
        const counts = [cardActionCount(snapshot, [head, merged]), cardActionCount(snapshot, [merged])];

        // then
        expect(counts).toEqual([3, 1]);
    });

    it("counts nothing for a card no action item was made from", () => {
        // given
        const snapshot = makeBoardSnapshot({ action_items: [makeActionItem({ source_card_id: null })] });

        // then
        expect(cardActionCount(snapshot, [makeCard({ id: "k1" })])).toBe(0);
    });
});

describe("actionCountLabel", () => {
    it("names one action and several actions", () => {
        // then
        expect([actionCountLabel(1), actionCountLabel(2)]).toEqual(["1 action", "2 actions"]);
    });
});

describe("sourceCard", () => {
    it("finds the card an action item was made from", () => {
        // given
        const card = makeCard({ id: "k1", body: "Standups ran long" });
        const snapshot = makeBoardSnapshot({ cards: [card] });

        // then
        expect(sourceCard(snapshot, makeActionItem({ source_card_id: "k1" }))).toEqual(card);
    });

    it("finds nothing for an item with no card, or a card the viewer cannot see", () => {
        // given
        const snapshot = makeBoardSnapshot({ cards: [makeCard({ id: "k1" })] });

        // when
        const found = [
            sourceCard(snapshot, makeActionItem({ source_card_id: null })),
            sourceCard(snapshot, makeActionItem({ source_card_id: "k9" })),
        ];

        // then
        expect(found).toEqual([null, null]);
    });
});

describe("revealedCardId", () => {
    it("reveals a merged card through the card it was merged into", () => {
        // then
        expect(revealedCardId(makeCard({ id: "k2", parent_card_id: "k1" }))).toBe("k1");
        expect(revealedCardId(makeCard({ id: "k1", parent_card_id: null }))).toBe("k1");
    });
});

describe("nextStatus", () => {
    it("marks an open item done, and reopens a done or dropped one", () => {
        // then
        expect(nextStatus("open")).toBe("done");
        expect(nextStatus("done")).toBe("open");
        expect(nextStatus("dropped")).toBe("open");
    });
});

describe("statusFilterOptions", () => {
    it("counts each status beside its filter, and every item beside All", () => {
        // when
        const options = statusFilterOptions({ open: 3, done: 2, dropped: 1 });

        // then
        expect(options.map(option => option.label)).toEqual(["All (6)", "Open (3)", "Done (2)", "Dropped (1)"]);
    });

    it("leaves the counts off until they are known", () => {
        // then
        expect(statusFilterOptions(undefined).map(option => option.label)).toEqual(["All", "Open", "Done", "Dropped"]);
    });
});

describe("completionLabel", () => {
    it("rounds the completion rate to a whole percentage", () => {
        // then
        expect(completionLabel(2 / 3)).toBe("67% complete");
        expect(completionLabel(0)).toBe("0% complete");
        expect(completionLabel(1)).toBe("100% complete");
    });

    it("says so when nothing is open or done yet", () => {
        // then
        expect(completionLabel(null)).toBe("Nothing to complete yet");
    });
});

describe("assigneeChoices", () => {
    it("offers anyone, the viewer and nobody, then each person assigned on the page by name", () => {
        // given
        const items = [
            makeActionItemListItem({ id: "i1", assignee: makePerson({ user_id: "u2", display_name: "Sam" }) }),
            makeActionItemListItem({ id: "i2", assignee: makePerson({ user_id: "u1", display_name: "Ali" }) }),
            makeActionItemListItem({ id: "i3", assignee: makePerson({ user_id: "u2", display_name: "Sam" }) }),
            makeActionItemListItem({ id: "i4", assignee: null }),
        ];

        // when
        const choices = assigneeChoices(items, null);

        // then
        expect(choices).toEqual([
            { value: "anyone", label: "Anyone" },
            { value: "me", label: "Assigned to me" },
            { value: "none", label: "Unassigned" },
            { value: "u1", label: "Ali" },
            { value: "u2", label: "Sam" },
        ]);
    });

    it("keeps the person picked even when the page holds none of their items", () => {
        // when
        const choices = assigneeChoices([], makePerson({ user_id: "u9", display_name: "Kit" }));

        // then
        expect(choices.at(-1)).toEqual({ value: "u9", label: "Kit" });
    });
});

describe("teamChoices", () => {
    it("offers every board, the organisation's own boards, then each team by name", () => {
        // when
        const choices = teamChoices([makeTeam({ id: "t2", name: "Ops" }), makeTeam({ id: "t1", name: "Crew" })]);

        // then
        expect(choices).toEqual([
            { value: "all", label: "All boards" },
            { value: "organisation", label: "Organisation boards" },
            { value: "t1", label: "Crew" },
            { value: "t2", label: "Ops" },
        ]);
    });
});

describe("pickedPerson", () => {
    it("finds the assignee a filter names, or nobody", () => {
        // given
        const ali = makePerson({ user_id: "u1", display_name: "Ali" });
        const items = [makeActionItemListItem({ assignee: ali })];

        // then
        expect(pickedPerson(items, "u1")).toEqual(ali);
        expect(pickedPerson(items, "u2")).toBeNull();
    });
});

describe("the page's filters", () => {
    it("sends no filter for All, Anyone and every board", () => {
        // then
        expect(statusQuery("all")).toBeUndefined();
        expect(assigneeQuery("anyone")).toBeUndefined();
        expect(teamQuery("all")).toBeUndefined();
    });

    it("sends every other choice as it is", () => {
        // then
        expect(statusQuery("done")).toBe("done");
        expect(assigneeQuery("me")).toBe("me");
        expect(assigneeQuery("u1")).toBe("u1");
        expect(teamQuery("organisation")).toBe("organisation");
    });
});
