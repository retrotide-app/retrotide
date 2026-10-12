import { describe, expect, it } from "vitest";
import { boardCountsByTeam } from "./teamBoardCounts";

describe("boardCountsByTeam", () => {
    it("counts the boards of each team", () => {
        // given
        const boards = [{ team_id: "team-crew" }, { team_id: "team-deck" }, { team_id: "team-crew" }];

        // when
        const counts = boardCountsByTeam(boards);

        // then
        expect(counts.get("team-crew")).toBe(2);
        expect(counts.get("team-deck")).toBe(1);
        expect(counts.size).toBe(2);
    });

    it("leaves out organisation boards, which have no team", () => {
        // given
        const boards = [{}, { team_id: "team-crew" }, { team_id: undefined }];

        // when
        const counts = boardCountsByTeam(boards);

        // then
        expect([...counts.entries()]).toEqual([["team-crew", 1]]);
    });

    it("answers an empty map for no boards", () => {
        // when
        const counts = boardCountsByTeam([]);

        // then
        expect(counts.size).toBe(0);
    });
});
