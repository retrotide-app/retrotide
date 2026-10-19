import { describe, expect, it } from "vitest";
import { joinBoardPath, sharedBoardPath, sharedBoardSummaryPath } from "./board/paths";
import { PRICING_PATH, PRICING_VISIBLE } from "./marketing";
import { routeTemplate, UNKNOWN_ROUTE_TEMPLATE } from "./routeTemplate";

describe("routeTemplate", () => {
    it("names a share link by its template, never by its slug", () => {
        // when
        const template = routeTemplate(joinBoardPath("sprint-review-a1b2c3"));

        // then
        expect(template).toBe("/b/:slug");
    });

    it("names a board inside an organisation by its template", () => {
        // when
        const templates = [
            routeTemplate("/o/harbour/boards/00000000-0000-4000-8000-0000000000b1"),
            routeTemplate("/o/harbour/boards/00000000-0000-4000-8000-0000000000b1/settings"),
            routeTemplate("/o/harbour/boards/00000000-0000-4000-8000-0000000000b1/summary"),
        ];

        // then
        expect(templates).toEqual([
            "/o/:orgSlug/boards/:boardId",
            "/o/:orgSlug/boards/:boardId/settings",
            "/o/:orgSlug/boards/:boardId/summary",
        ]);
    });

    it("prefers a literal segment over a parameter in the same place", () => {
        // when
        const template = routeTemplate("/o/harbour/boards/new");

        // then
        expect(template).toBe("/o/:orgSlug/boards/new");
    });

    it("names a shared board, its summary and its print view", () => {
        // given
        const boardId = "00000000-0000-4000-8000-0000000000b1";

        // when
        const templates = [
            routeTemplate(sharedBoardPath(boardId)),
            routeTemplate(sharedBoardSummaryPath(boardId)),
            routeTemplate(`${sharedBoardPath(boardId)}/print`),
        ];

        // then
        expect(templates).toEqual(["/boards/:boardId", "/boards/:boardId/summary", "/boards/:boardId/print"]);
    });

    it("keeps a fixed path as it is", () => {
        // when
        const templates = ["/", "/login", "/account", "/onboarding", "/two-factor", "/admin", "/admin/audit-log"].map(
            routeTemplate,
        );

        // then
        expect(templates).toEqual([
            "/",
            "/login",
            "/account",
            "/onboarding",
            "/two-factor",
            "/admin",
            "/admin/audit-log",
        ]);
    });

    it("names the admin and marketing pages that carry a parameter", () => {
        // when
        const templates = [
            routeTemplate("/admin/users/00000000-0000-4000-8000-0000000000a1"),
            routeTemplate("/admin/organisations/00000000-0000-4000-8000-0000000000c1"),
            routeTemplate("/templates/start-stop-continue"),
            routeTemplate("/legal/privacy"),
            routeTemplate("/o/harbour/teams/00000000-0000-4000-8000-0000000000d1"),
        ];

        // then
        expect(templates).toEqual([
            "/admin/users/:userId",
            "/admin/organisations/:orgId",
            "/templates/:key",
            "/legal/:page",
            "/o/:orgSlug/teams/:teamId",
        ]);
    });

    it("ignores a trailing slash and the case of a literal segment, as the router does", () => {
        // when
        const templates = [routeTemplate("/o/harbour/members/"), routeTemplate("/LOGIN")];

        // then
        expect(templates).toEqual(["/o/:orgSlug/members", "/login"]);
    });

    it("names the pricing page only while it is mounted", () => {
        // when
        const template = routeTemplate(PRICING_PATH);

        // then
        expect(template).toBe(PRICING_VISIBLE ? PRICING_PATH : UNKNOWN_ROUTE_TEMPLATE);
    });

    it("calls a path the app has no route for unknown, so nothing concrete leaves the browser", () => {
        // when
        const templates = [
            routeTemplate("/b"),
            routeTemplate("/b/sprint-review/extra"),
            routeTemplate("/o/harbour/nowhere"),
            routeTemplate("/admin/users/a1/b2"),
            routeTemplate("/no-such-page"),
        ];

        // then
        expect(templates).toEqual([
            UNKNOWN_ROUTE_TEMPLATE,
            UNKNOWN_ROUTE_TEMPLATE,
            UNKNOWN_ROUTE_TEMPLATE,
            UNKNOWN_ROUTE_TEMPLATE,
            UNKNOWN_ROUTE_TEMPLATE,
        ]);
        expect(UNKNOWN_ROUTE_TEMPLATE).toBe("unknown");
    });
});
