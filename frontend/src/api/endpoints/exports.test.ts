import { describe, expect, it } from "vitest";
import { boardExportUrl, organisationExportUrl, SELF_EXPORT_URL } from "./exports";

describe("export downloads", () => {
    it("asks for a board export in the chosen format", () => {
        // given
        const boardId = "00000000-0000-4000-8000-0000000000b1";

        // when
        const urls = [
            boardExportUrl(boardId, "csv"),
            boardExportUrl(boardId, "markdown"),
            boardExportUrl(boardId, "json"),
        ];

        // then
        expect(urls).toEqual([
            `/api/v1/boards/${boardId}/export?format=csv`,
            `/api/v1/boards/${boardId}/export?format=markdown`,
            `/api/v1/boards/${boardId}/export?format=json`,
        ]);
    });

    it("escapes the board ID so it cannot add a segment", () => {
        // when
        const url = boardExportUrl("a/b", "csv");

        // then
        expect(url).toBe("/api/v1/boards/a%2Fb/export?format=csv");
    });

    it("asks for an organisation export by the organisation's ID", () => {
        // when
        const url = organisationExportUrl("org-harbour");

        // then
        expect(url).toBe("/api/v1/orgs/org-harbour/export");
    });

    it("asks for the signed-in user's own data", () => {
        // then
        expect(SELF_EXPORT_URL).toBe("/api/v1/auth/export");
    });
});
