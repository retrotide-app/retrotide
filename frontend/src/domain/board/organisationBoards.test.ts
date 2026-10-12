import { describe, expect, it } from "vitest";
import { makeOrgBoardListItem, makeTeam } from "../../test-utils/fixtures";
import {
    splitEnded,
    type BoardQuery,
    boardCountLabels,
    boardCounts,
    boardHome,
    boardSections,
    boardSort,
    boardStatus,
    boardTeamOptions,
    bulkFailures,
    DEFAULT_BOARD_QUERY,
    isFiltering,
    moveTargets,
    ORGANISATION_SECTION_TITLE,
    PINNED_SECTION_TITLE,
    shownBoards,
    toggledSelection,
} from "./organisationBoards";

const crew = makeTeam({ id: "t1", name: "Crew", role: "member" });

const deck = makeTeam({ id: "t2", name: "Deck", role: undefined });

const stores = makeTeam({ id: "t3", name: "Stores", role: "lead", archived_at: "2026-09-20T09:00:00Z" });

const townHall = makeOrgBoardListItem({ id: "b1", team_id: undefined, team_name: undefined, title: "Town hall" });

const sprint = makeOrgBoardListItem({ id: "b2", team_id: "t1", team_name: "Crew", title: "Sprint 12" });

const launch = makeOrgBoardListItem({ id: "b3", team_id: "t2", team_name: "Deck", title: "Launch review" });

const retired = makeOrgBoardListItem({
    id: "b4",
    team_id: "t1",
    team_name: "Crew",
    title: "Sprint 11",
    archived_at: "2026-09-30T09:00:00Z",
});

function query(overrides: Partial<BoardQuery> = {}): BoardQuery {
    return { ...DEFAULT_BOARD_QUERY, ...overrides };
}

describe("splitEnded", () => {
    it("keeps live boards apart from ended retros, in their order", () => {
        // given
        const live = makeOrgBoardListItem({ id: "b1" });
        const ended = makeOrgBoardListItem({ id: "b2", ended: true });
        const later = makeOrgBoardListItem({ id: "b3" });

        // when
        const split = splitEnded([live, ended, later]);

        // then
        expect(split.live.map(board => board.id)).toEqual(["b1", "b3"]);
        expect(split.ended.map(board => board.id)).toEqual(["b2"]);
    });
});

describe("boardCounts", () => {
    it("counts live and archived boards apart", () => {
        // when
        const counts = boardCounts([townHall, sprint, retired]);

        // then
        expect(counts).toEqual({ live: 2, archived: 1 });
    });

    it("counts nothing for no boards", () => {
        // when
        const counts = boardCounts([]);

        // then
        expect(counts).toEqual({ live: 0, archived: 0 });
    });
});

describe("isFiltering", () => {
    it("treats blank text as no filter", () => {
        // when
        const filtering = [isFiltering(query({ text: "   " })), isFiltering(query({ text: "spr" }))];

        // then
        expect(filtering).toEqual([false, true]);
    });

    it("counts a team, a status other than active, and pinned only as filters", () => {
        // when
        const filtering = [
            isFiltering(query({ team: "organisation" })),
            isFiltering(query({ status: "all" })),
            isFiltering(query({ pinnedOnly: true })),
            isFiltering(query()),
        ];

        // then
        expect(filtering).toEqual([true, true, true, false]);
    });
});

describe("filtering by status, team and pin", () => {
    const ended = makeOrgBoardListItem({ id: "b5", team_id: "t1", team_name: "Crew", title: "Sprint 10", ended: true });
    const pinnedLaunch = { ...launch, pinned: true as const };
    const boards = [townHall, sprint, pinnedLaunch, retired, ended];

    it.each([
        { status: "active" as const, ids: ["b1", "b2", "b3"] },
        { status: "ended" as const, ids: ["b5"] },
        { status: "archived" as const, ids: ["b4"] },
        { status: "all" as const, ids: ["b1", "b2", "b3", "b4", "b5"] },
        { status: "deleted" as const, ids: [] },
    ])("keeps the $status boards", ({ status, ids }) => {
        // when
        const shown = shownBoards(boards, query({ status }));

        // then
        expect(shown.map(board => board.id)).toEqual(ids);
    });

    it.each([
        { team: "all", ids: ["b1", "b2", "b3"] },
        { team: "organisation", ids: ["b1"] },
        { team: "t1", ids: ["b2"] },
    ])("keeps the boards of team $team", ({ team, ids }) => {
        // when
        const shown = shownBoards(boards, query({ team }));

        // then
        expect(shown.map(board => board.id)).toEqual(ids);
    });

    it("keeps only pinned boards when asked", () => {
        // when
        const shown = shownBoards(boards, query({ pinnedOnly: true }));

        // then
        expect(shown.map(board => board.id)).toEqual(["b3"]);
    });
});

describe("boardStatus", () => {
    it("reads a known status and refuses anything else", () => {
        // when
        const statuses = [boardStatus("active"), boardStatus("deleted"), boardStatus("gone")];

        // then
        expect(statuses).toEqual(["active", "deleted", undefined]);
    });
});

describe("boardTeamOptions", () => {
    it("offers every board, the organisation's boards, then each team the viewer can see by name", () => {
        // when
        const options = boardTeamOptions([deck, crew, stores], "member");

        // then
        expect(options).toEqual([
            { value: "all", label: "All teams" },
            { value: "organisation", label: "Organisation boards" },
            { value: "t1", label: "Crew" },
            { value: "t3", label: "Stores" },
        ]);
    });
});

describe("boardHome", () => {
    it("names a board's team, or the organisation for a board with none", () => {
        // when
        const homes = [boardHome({ team_id: "t1" }), boardHome({}), boardHome({ team_id: "" })];

        // then
        expect(homes).toEqual(["t1", "organisation", "organisation"]);
    });
});

describe("moveTargets", () => {
    it("offers the organisation and every live team the viewer may create boards on, by name", () => {
        // when
        const targets = moveTargets([deck, crew, stores], "admin", "t1");

        // then
        expect(targets).toEqual([
            { value: "organisation", label: "Organisation board (no team)" },
            { value: "t1", label: "Crew" },
            { value: "t2", label: "Deck" },
        ]);
    });

    it("gives a member their own teams and the organisation", () => {
        // when
        const targets = moveTargets([deck, crew, stores], "member", "organisation");

        // then
        expect(targets.map(target => target.value)).toEqual(["organisation", "t1"]);
    });

    it("keeps the board's current team first when the viewer could not create a board there", () => {
        // when
        const targets = moveTargets([deck, crew], "member", "t2");

        // then
        expect(targets).toEqual([
            { value: "t2", label: "Deck" },
            { value: "organisation", label: "Organisation board (no team)" },
            { value: "t1", label: "Crew" },
        ]);
    });

    it("names a current team it does not know", () => {
        // when
        const targets = moveTargets([], null, "t9");

        // then
        expect(targets).toEqual([{ value: "t9", label: "Its current team" }]);
    });
});

describe("toggledSelection", () => {
    it("adds a board that was not chosen and drops one that was, leaving the old set alone", () => {
        // given
        const chosen = new Set(["b1"]);

        // when
        const added = toggledSelection(chosen, "b2");
        const dropped = toggledSelection(chosen, "b1");

        // then
        expect([...added]).toEqual(["b1", "b2"]);
        expect([...dropped]).toEqual([]);
        expect([...chosen]).toEqual(["b1"]);
    });
});

describe("bulkFailures", () => {
    it("names each board a bulk action could not change, with the reason", () => {
        // when
        const failures = bulkFailures(
            [
                { board_id: "b1", ok: true },
                { board_id: "b2", ok: false, error: "your role on this board does not allow that" },
                { board_id: "gone", ok: false, error: "board not found" },
            ],
            [townHall, sprint],
        );

        // then
        expect(failures).toEqual([
            { boardId: "b2", title: "Sprint 12", reason: "your role on this board does not allow that" },
            { boardId: "gone", title: "gone", reason: "board not found" },
        ]);
    });
});

describe("shownBoards", () => {
    const boards = [townHall, sprint, launch, retired];

    it("keeps the server's newest first order and leaves archived boards out by default", () => {
        // when
        const shown = shownBoards(boards, query());

        // then
        expect(shown.map(board => board.id)).toEqual(["b1", "b2", "b3"]);
    });

    it("turns the order round for oldest first", () => {
        // when
        const shown = shownBoards(boards, query({ sort: "oldest", status: "all" }));

        // then
        expect(shown.map(board => board.id)).toEqual(["b4", "b3", "b2", "b1"]);
    });

    it("matches the title in any case, ignoring spaces around the text", () => {
        // when
        const shown = shownBoards(boards, query({ text: "  SPRINT ", status: "all" }));

        // then
        expect(shown.map(board => board.id)).toEqual(["b2", "b4"]);
    });

    it("leaves the boards it was given untouched", () => {
        // given
        const given = [townHall, sprint];

        // when
        shownBoards(given, query({ sort: "oldest" }));

        // then
        expect(given).toEqual([townHall, sprint]);
    });
});

describe("boardSections", () => {
    it("puts the organisation boards first, then one section per team by name", () => {
        // when
        const sections = boardSections([launch, sprint, townHall], [deck, crew], "owner", query());

        // then
        expect(sections.map(section => [section.title, section.boards.map(board => board.id)])).toEqual([
            [ORGANISATION_SECTION_TITLE, ["b1"]],
            ["Crew", ["b2"]],
            ["Deck", ["b3"]],
        ]);
        expect(sections[0]).toMatchObject({ key: "organisation", teamId: null, canAdd: true });
        expect(sections[1]).toMatchObject({ key: "t1", teamId: "t1", canAdd: true, archived: false });
    });

    it("lets every member add an organisation board, and a team board only in their own live teams", () => {
        // when
        const sections = boardSections([], [crew, deck, stores], "member", query());

        // then
        expect(sections.map(section => [section.title, section.canAdd])).toEqual([
            [ORGANISATION_SECTION_TITLE, true],
            ["Crew", true],
        ]);
    });

    it("gives an admin a section with an add tile for every live team", () => {
        // when
        const sections = boardSections([], [crew, deck, stores], "admin", query());

        // then
        expect(sections.map(section => section.title)).toEqual([ORGANISATION_SECTION_TITLE, "Crew", "Deck"]);
    });

    it("keeps an archived team's section while it has boards, with no add tile", () => {
        // given
        const old = makeOrgBoardListItem({ id: "b9", team_id: "t3", team_name: "Stores", title: "Stocktake" });

        // when
        const sections = boardSections([old], [stores], "owner", query());

        // then
        expect(sections[1]).toMatchObject({ title: "Stores", archived: true, canAdd: false });
        expect(sections[1].boards).toEqual([old]);
    });

    it("names a section after the board's team when the team list does not hold it", () => {
        // when
        const sections = boardSections([launch], [], "member", query());

        // then
        expect(sections.map(section => [section.title, section.canAdd])).toEqual([
            [ORGANISATION_SECTION_TITLE, true],
            ["Deck", false],
        ]);
    });

    it("drops every section without a match while filtering, add tiles included", () => {
        // when
        const sections = boardSections([townHall, sprint, launch], [crew, deck], "owner", query({ text: "launch" }));

        // then
        expect(sections.map(section => section.title)).toEqual(["Deck"]);
        expect(sections[0].canAdd).toBe(false);
    });

    it("hides archived boards unless asked for them", () => {
        // when
        const hidden = boardSections([retired], [crew], "member", query());
        const shown = boardSections([retired], [crew], "member", query({ status: "archived" }));

        // then
        expect(hidden[1].boards).toEqual([]);
        expect(shown.map(section => [section.title, section.boards])).toEqual([["Crew", [retired]]]);
    });

    it("puts the pinned boards first in their own section, and keeps them in their team's section too", () => {
        // given
        const pinnedSprint = { ...sprint, pinned: true as const };

        // when
        const sections = boardSections([townHall, pinnedSprint, launch], [crew, deck], "owner", query());

        // then
        expect(sections.map(section => [section.title, section.boards.map(board => board.id)])).toEqual([
            [PINNED_SECTION_TITLE, ["b2"]],
            [ORGANISATION_SECTION_TITLE, ["b1"]],
            ["Crew", ["b2"]],
            ["Deck", ["b3"]],
        ]);
        expect(sections[0]).toMatchObject({ key: "pinned", canAdd: false });
    });

    it("shows only the pinned section when only pinned boards are wanted", () => {
        // given
        const pinnedSprint = { ...sprint, pinned: true as const };

        // when
        const sections = boardSections([townHall, pinnedSprint], [crew], "owner", query({ pinnedOnly: true }));
        const none = boardSections([townHall], [crew], "owner", query({ pinnedOnly: true }));

        // then
        expect(sections.map(section => [section.title, section.boards.map(board => board.id)])).toEqual([
            [PINNED_SECTION_TITLE, ["b2"]],
        ]);
        expect(none).toEqual([]);
    });

    it("keeps only the chosen team's section, with no add tiles", () => {
        // when
        const sections = boardSections([townHall, sprint, launch], [crew, deck], "owner", query({ team: "t2" }));

        // then
        expect(sections.map(section => [section.title, section.canAdd])).toEqual([["Deck", false]]);
    });

    it("offers nothing to someone with no role in the organisation", () => {
        // when
        const sections = boardSections([], [crew], null, query());

        // then
        expect(sections).toEqual([]);
    });
});

describe("boardCountLabels", () => {
    it("names the live boards and, when there are any, the archived ones", () => {
        // when
        const labels = [
            boardCountLabels({ live: 3, archived: 1 }),
            boardCountLabels({ live: 1, archived: 0 }),
            boardCountLabels({ live: 0, archived: 2 }),
        ];

        // then
        expect(labels).toEqual([["3 boards", "1 archived"], ["1 board"], ["0 boards", "2 archived"]]);
    });
});

describe("boardSort", () => {
    it("reads a known order and refuses anything else", () => {
        // when
        const sorts = [boardSort("newest"), boardSort("oldest"), boardSort("sideways")];

        // then
        expect(sorts).toEqual(["newest", "oldest", undefined]);
    });
});

describe("section hints", () => {
    it("says who can open the boards in each section", () => {
        // given
        const old = makeOrgBoardListItem({ id: "b9", team_id: "t3", team_name: "Stores", title: "Stocktake" });

        // when
        const sections = boardSections([townHall, sprint, old], [crew, stores], "owner", query());

        // then
        expect(sections.map(section => section.hint)).toEqual([
            "Everyone in the organisation can open these and add cards.",
            "Only people on this team can open these.",
            "This team is archived, so nobody can add boards to it.",
        ]);
    });
});
