import { describe, expect, it } from "vitest";
import { makeTeam } from "../../test-utils/fixtures";
import {
    creatableTeams,
    createsOrganisationBoards,
    initialTeamChoice,
    NO_TEAM,
    newBoardSteps,
    presetsNoTeam,
    presetTeam,
} from "./newBoard";
import { NEW_BOARD_NO_TEAM } from "./paths";

const crew = makeTeam({ id: "team-crew", name: "Crew", role: "member" });
const deck = makeTeam({ id: "team-deck", name: "Deck", role: "lead" });
const galley = makeTeam({ id: "team-galley", name: "Galley", role: undefined });
const stores = makeTeam({ id: "team-stores", name: "Stores", role: "lead", archived_at: "2026-09-10T09:00:00Z" });

describe("creatableTeams", () => {
    it("keeps the live teams a plain member belongs to", () => {
        // when
        const teams = creatableTeams([crew, deck, galley, stores], "member");

        // then
        expect(teams.map(team => team.id)).toEqual(["team-crew", "team-deck"]);
    });

    it("keeps every live team for an organisation admin, member of it or not", () => {
        // when
        const teams = creatableTeams([crew, deck, galley, stores], "admin");

        // then
        expect(teams.map(team => team.id)).toEqual(["team-crew", "team-deck", "team-galley"]);
    });

    it("keeps nothing for a viewer with no organisation role", () => {
        // when
        const teams = creatableTeams([crew, deck], null);

        // then
        expect(teams).toEqual([]);
    });
});

describe("createsOrganisationBoards", () => {
    it.each([
        { role: "owner", expected: true },
        { role: "admin", expected: true },
        { role: "member", expected: true },
        { role: null, expected: false },
    ] as const)("lets a $role create a board with no team: $expected", ({ role, expected }) => {
        // when
        const allowed = createsOrganisationBoards(role);

        // then
        expect(allowed).toBe(expected);
    });
});

describe("presetTeam", () => {
    it("takes the requested team when the viewer may create in it", () => {
        // when
        const team = presetTeam([crew, deck], "team-deck");

        // then
        expect(team).toBe(deck);
    });

    it("ignores a requested team the viewer may not create in", () => {
        // when
        const team = presetTeam([crew, deck], "team-stores");

        // then
        expect(team).toBeNull();
    });

    it("leaves the choice open when no team is requested, even with just one team", () => {
        // when
        const team = presetTeam([crew], null);

        // then
        expect(team).toBeNull();
    });
});

describe("presetsNoTeam", () => {
    it("takes no team as chosen when the address asks for it and the viewer may create organisation boards", () => {
        // when
        const preset = presetsNoTeam(NEW_BOARD_NO_TEAM, true);

        // then
        expect(preset).toBe(true);
    });

    it("leaves the choice open when the viewer may not create organisation boards", () => {
        // when
        const preset = presetsNoTeam(NEW_BOARD_NO_TEAM, false);

        // then
        expect(preset).toBe(false);
    });

    it("leaves the choice open when the address names a team or nothing", () => {
        // when
        const named = presetsNoTeam("team-crew", true);
        const absent = presetsNoTeam(null, true);

        // then
        expect(named).toBe(false);
        expect(absent).toBe(false);
    });
});

describe("initialTeamChoice", () => {
    it("starts on the preset team", () => {
        // when
        const choice = initialTeamChoice([crew, deck], deck, true);

        // then
        expect(choice).toBe("team-deck");
    });

    it("starts on no team when the viewer may create organisation boards", () => {
        // when
        const choice = initialTeamChoice([crew, deck], null, true);

        // then
        expect(choice).toBe(NO_TEAM);
    });

    it("starts on the first team when organisation boards are not open to the viewer", () => {
        // when
        const choice = initialTeamChoice([crew, deck], null, false);

        // then
        expect(choice).toBe("team-crew");
    });
});

describe("newBoardSteps", () => {
    it("asks for the team first when none is preset", () => {
        // when
        const steps = newBoardSteps(false);

        // then
        expect(steps).toEqual(["team", "template", "details"]);
    });

    it("skips the team step when the team is preset", () => {
        // when
        const steps = newBoardSteps(true);

        // then
        expect(steps).toEqual(["template", "details"]);
    });
});
