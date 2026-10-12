import { describe, expect, it } from "vitest";
import { queryKeys } from "./queryKeys";

function startsWith(key: readonly unknown[], prefix: readonly unknown[]): boolean {
    return prefix.every((part, index) => key[index] === part);
}

describe("queryKeys literal output", () => {
    it("builds the site info key", () => {
        // when
        const key = queryKeys.siteInfo();

        // then
        expect(key).toEqual(["site-info"]);
    });

    it("roots every auth key under auth", () => {
        // then
        expect(queryKeys.auth.all).toEqual(["auth"]);
    });

    it("builds the session key under the auth root", () => {
        // when
        const key = queryKeys.auth.session();

        // then
        expect(key).toEqual(["auth", "session"]);
    });

    it("builds the linked identities key under the auth root", () => {
        // when
        const key = queryKeys.auth.identities();

        // then
        expect(key).toEqual(["auth", "identities"]);
    });

    it("builds an invitation key with the invitation's token under the auth root", () => {
        // when
        const key = queryKeys.auth.invitation("invite-1");

        // then
        expect(key).toEqual(["auth", "invitation", "invite-1"]);
    });

    it("builds the two-factor status key under the auth root", () => {
        // when
        const key = queryKeys.auth.twoFactor();

        // then
        expect(key).toEqual(["auth", "two-factor"]);
    });

    it("builds an organisation's saved templates key under that organisation's scope", () => {
        // when
        const key = queryKeys.organisations.savedTemplates("o1");

        // then
        expect(key).toEqual(["organisations", "org", "o1", "saved-templates"]);
        expect(key.slice(0, 3)).toEqual(queryKeys.organisations.scope("o1"));
    });

    it("builds an organisation's recently deleted boards key under that organisation's scope", () => {
        // when
        const key = queryKeys.organisations.deletedBoards("o1");

        // then
        expect(key).toEqual(["organisations", "org", "o1", "deleted-boards"]);
        expect(key.slice(0, 3)).toEqual(queryKeys.organisations.scope("o1"));
    });

    it("builds the survey template catalogue key outside the board template catalogue", () => {
        // when
        const key = queryKeys.surveyTemplates();

        // then
        expect(key).toEqual(["survey-templates"]);
        expect(key[0]).not.toBe(queryKeys.boardTemplates()[0]);
    });

    it("roots every admin key under admin", () => {
        // then
        expect(queryKeys.admin.all).toEqual(["admin"]);
        expect(queryKeys.admin.users.all).toEqual(["admin", "users"]);
        expect(queryKeys.admin.invitations.all).toEqual(["admin", "invitations"]);
        expect(queryKeys.admin.auditLog.all).toEqual(["admin", "audit-log"]);
    });

    it("builds the admin invitation list key with its page", () => {
        // when
        const key = queryKeys.admin.invitations.list({ limit: 20, offset: 40 });

        // then
        expect(key).toEqual(["admin", "invitations", "list", { limit: 20, offset: 40 }]);
    });

    it("builds the admin user list key with its search and page", () => {
        // when
        const key = queryKeys.admin.users.list({ search: "sam", limit: 20, offset: 40 });

        // then
        expect(key).toEqual(["admin", "users", "list", { search: "sam", limit: 20, offset: 40 }]);
    });

    it("builds the admin organisation list key with its search and page under the organisations root", () => {
        // when
        const key = queryKeys.admin.organisations.list({ search: "acme", limit: 20, offset: 40 });

        // then
        expect(queryKeys.admin.organisations.all).toEqual(["admin", "organisations"]);
        expect(key).toEqual(["admin", "organisations", "list", { search: "acme", limit: 20, offset: 40 }]);
    });

    it("builds the admin organisation detail key with the organisation's id", () => {
        // when
        const key = queryKeys.admin.organisations.detail("o1");

        // then
        expect(key).toEqual(["admin", "organisations", "detail", "o1"]);
    });

    it("builds the admin user detail key with the user's id", () => {
        // when
        const key = queryKeys.admin.users.detail("u1");

        // then
        expect(key).toEqual(["admin", "users", "detail", "u1"]);
    });

    it("builds a user's audit log key with the user's id and the page", () => {
        // when
        const key = queryKeys.admin.users.auditLog("u1", { limit: 50, offset: 0 });

        // then
        expect(key).toEqual(["admin", "users", "audit-log", "u1", { limit: 50, offset: 0 }]);
    });

    it("builds the key that holds every page of a user's audit log", () => {
        // when
        const key = queryKeys.admin.users.auditLogPages("u1");

        // then
        expect(key).toEqual(["admin", "users", "audit-log", "u1"]);
    });

    it("builds the settings and permissions keys", () => {
        // then
        expect(queryKeys.admin.settings()).toEqual(["admin", "settings"]);
        expect(queryKeys.admin.permissions()).toEqual(["admin", "permissions"]);
    });

    it("builds the site stats key under the admin root", () => {
        // when
        const key = queryKeys.admin.stats();

        // then
        expect(key).toEqual(["admin", "stats"]);
        expect(startsWith(key, queryKeys.admin.all)).toBe(true);
    });

    it("builds the site audit log key with its filter and page", () => {
        // when
        const key = queryKeys.admin.auditLog.list({ action: "ban_user", limit: 50, offset: 0 });

        // then
        expect(key).toEqual(["admin", "audit-log", "list", { action: "ban_user", limit: 50, offset: 0 }]);
    });

    it("builds the organisation list key under the organisations root", () => {
        // when
        const key = queryKeys.organisations.list();

        // then
        expect(queryKeys.organisations.all).toEqual(["organisations"]);
        expect(key).toEqual(["organisations", "list"]);
    });

    it("builds every key of one organisation under that organisation's scope", () => {
        // then
        expect(queryKeys.organisations.scope("o1")).toEqual(["organisations", "org", "o1"]);
        expect(queryKeys.organisations.detail("o1")).toEqual(["organisations", "org", "o1", "detail"]);
        expect(queryKeys.organisations.members("o1")).toEqual(["organisations", "org", "o1", "members"]);
        expect(queryKeys.organisations.teams("o1")).toEqual(["organisations", "org", "o1", "teams"]);
        expect(queryKeys.organisations.invites("o1")).toEqual(["organisations", "org", "o1", "invites"]);
        expect(queryKeys.organisations.entitlements("o1")).toEqual(["organisations", "org", "o1", "entitlements"]);
    });

    it("builds every key of one team under that team's scope", () => {
        // then
        expect(queryKeys.teams.all).toEqual(["teams"]);
        expect(queryKeys.teams.scope("t1")).toEqual(["teams", "t1"]);
        expect(queryKeys.teams.detail("t1")).toEqual(["teams", "t1", "detail"]);
        expect(queryKeys.teams.members("t1")).toEqual(["teams", "t1", "members"]);
    });

    it("builds an invite lookup key with the invite's token", () => {
        // when
        const key = queryKeys.invites.lookup("invite-1");

        // then
        expect(queryKeys.invites.all).toEqual(["invites"]);
        expect(key).toEqual(["invites", "lookup", "invite-1"]);
    });

    it("builds a team's board list key under the boards root", () => {
        // when
        const key = queryKeys.boards.team("t1");

        // then
        expect(queryKeys.boards.all).toEqual(["boards"]);
        expect(key).toEqual(["boards", "team", "t1"]);
    });

    it("builds an organisation's board list key under the organisation boards prefix", () => {
        // when
        const key = queryKeys.boards.organisation("o1");

        // then
        expect(queryKeys.boards.organisations).toEqual(["boards", "org"]);
        expect(key).toEqual(["boards", "org", "o1"]);
        expect(startsWith(key, queryKeys.boards.organisations)).toBe(true);
        expect(startsWith(key, queryKeys.boards.all)).toBe(true);
        expect(startsWith(queryKeys.boards.team("o1"), queryKeys.boards.organisations)).toBe(false);
    });

    it("builds every live key of one board under the live root", () => {
        // then
        expect(queryKeys.boardLive.all).toEqual(["board-live"]);
        expect(queryKeys.boardLive.state("b1")).toEqual(["board-live", "b1", "state"]);
        expect(queryKeys.boardLive.mutations("b1")).toEqual(["board-live", "b1", "mutations"]);
    });

    it("builds the board template catalogue key", () => {
        // when
        const key = queryKeys.boardTemplates();

        // then
        expect(key).toEqual(["board-templates"]);
    });

    it("builds one template's key under the catalogue key", () => {
        // when
        const key = queryKeys.boardTemplate("sailboat");

        // then
        expect(key).toEqual(["board-templates", "sailboat"]);
        expect(startsWith(key, queryKeys.boardTemplates())).toBe(true);
    });

    it("keeps a board's sharing outside its live state, so a board event never refetches the sharing", () => {
        // when
        const key = queryKeys.boardSharing("b1");

        // then
        expect(key).toEqual(["board-sharing", "b1"]);
        expect(startsWith(key, queryKeys.boardLive.all)).toBe(false);
    });

    it("keeps a board's people outside its live state, so a board event never refetches the people", () => {
        // when
        const key = queryKeys.boardPeople("b1");

        // then
        expect(key).toEqual(["board-people", "b1"]);
        expect(startsWith(key, queryKeys.boardLive.all)).toBe(false);
    });

    it("builds an organisation's action item list key with its filters under that organisation's prefix", () => {
        // when
        const key = queryKeys.actionItems.list("o1", { status: "open", assignee: "me", offset: 50 });

        // then
        expect(queryKeys.actionItems.all).toEqual(["action-items"]);
        expect(queryKeys.actionItems.organisation("o1")).toEqual(["action-items", "o1"]);
        expect(key).toEqual(["action-items", "o1", { status: "open", assignee: "me", offset: 50 }]);
        expect(startsWith(key, queryKeys.actionItems.organisation("o1"))).toBe(true);
        expect(startsWith(key, queryKeys.actionItems.organisation("o2"))).toBe(false);
        expect(startsWith(key, queryKeys.actionItems.all)).toBe(true);
    });

    it("builds the signed in user's mentions key under the auth root", () => {
        // when
        const key = queryKeys.auth.mentions();

        // then
        expect(key).toEqual(["auth", "mentions"]);
        expect(startsWith(key, queryKeys.auth.all)).toBe(true);
    });

    it("builds a share link's preview key from its slug", () => {
        // when
        const key = queryKeys.shareLinks.preview("k7Q2");

        // then
        expect(key).toEqual(["share-links", "k7Q2", "preview"]);
    });

    it("builds the guest claim offer key per user under the guest claim prefix", () => {
        // when
        const key = queryKeys.auth.guestClaim("u1");

        // then
        expect(key).toEqual(["auth", "guest-claim", "u1"]);
        expect(startsWith(key, queryKeys.auth.guestClaims)).toBe(true);
    });
});

describe("queryKeys roots", () => {
    it("gives every root a distinct value so invalidation cannot bleed across them", () => {
        // given
        const roots = [
            queryKeys.siteInfo()[0],
            queryKeys.auth.all[0],
            queryKeys.admin.all[0],
            queryKeys.organisations.all[0],
            queryKeys.teams.all[0],
            queryKeys.invites.all[0],
            queryKeys.boards.all[0],
            queryKeys.boardLive.all[0],
            queryKeys.boardSharing("b1")[0],
            queryKeys.boardPeople("b1")[0],
            queryKeys.actionItems.all[0],
            queryKeys.shareLinks.preview("k7Q2")[0],
            queryKeys.boardTemplates()[0],
            queryKeys.surveyTemplates()[0],
        ];

        // when
        const unique = new Set(roots);

        // then
        expect(unique.size).toBe(roots.length);
    });

    it("starts the session and identities keys with the auth root so invalidating auth reaches them", () => {
        // given
        const root = queryKeys.auth.all;

        // then
        expect(startsWith(queryKeys.auth.session(), root)).toBe(true);
        expect(startsWith(queryKeys.auth.identities(), root)).toBe(true);
        expect(startsWith(queryKeys.auth.invitation("invite-1"), root)).toBe(true);
        expect(startsWith(queryKeys.auth.guestClaim("u1"), root)).toBe(true);
    });

    it("starts every invitation list page with the invitations root so one invalidation refreshes them all", () => {
        // given
        const root = queryKeys.admin.invitations.all;

        // then
        expect(startsWith(queryKeys.admin.invitations.list({ limit: 20, offset: 20 }), root)).toBe(true);
    });

    it("starts every user key with the users root so one invalidation refreshes the list, a record and its log", () => {
        // given
        const root = queryKeys.admin.users.all;

        // then
        expect(startsWith(queryKeys.admin.users.list({}), root)).toBe(true);
        expect(startsWith(queryKeys.admin.users.detail("u1"), root)).toBe(true);
        expect(startsWith(queryKeys.admin.users.auditLog("u1", {}), root)).toBe(true);
    });

    it("starts every page of a user's audit log with that user's audit log prefix", () => {
        // given
        const prefix = queryKeys.admin.users.auditLogPages("u1");

        // then
        expect(startsWith(queryKeys.admin.users.auditLog("u1", { limit: 50, offset: 50 }), prefix)).toBe(true);
        expect(startsWith(queryKeys.admin.users.auditLog("u2", {}), prefix)).toBe(false);
    });

    it("keeps a user's own audit log out of the site audit log root", () => {
        // given
        const siteLog = queryKeys.admin.auditLog.all;

        // when
        const userLog = queryKeys.admin.users.auditLog("u1", {});

        // then
        expect(startsWith(userLog, siteLog)).toBe(false);
    });

    it("starts the site audit log list with its root", () => {
        // then
        expect(startsWith(queryKeys.admin.auditLog.list({}), queryKeys.admin.auditLog.all)).toBe(true);
    });

    it("starts every admin key with the admin root", () => {
        // given
        const root = queryKeys.admin.all;

        // then
        expect(startsWith(queryKeys.admin.settings(), root)).toBe(true);
        expect(startsWith(queryKeys.admin.permissions(), root)).toBe(true);
        expect(startsWith(queryKeys.admin.users.all, root)).toBe(true);
        expect(startsWith(queryKeys.admin.invitations.all, root)).toBe(true);
        expect(startsWith(queryKeys.admin.auditLog.all, root)).toBe(true);
    });

    it("starts every key of one organisation with its scope, so one invalidation reaches them all", () => {
        // given
        const scope = queryKeys.organisations.scope("o1");

        // then
        expect(startsWith(queryKeys.organisations.detail("o1"), scope)).toBe(true);
        expect(startsWith(queryKeys.organisations.members("o1"), scope)).toBe(true);
        expect(startsWith(queryKeys.organisations.teams("o1"), scope)).toBe(true);
        expect(startsWith(queryKeys.organisations.invites("o1"), scope)).toBe(true);
        expect(startsWith(queryKeys.organisations.entitlements("o1"), scope)).toBe(true);
    });

    it("keeps one organisation's keys out of another organisation's scope and out of the list", () => {
        // given
        const other = queryKeys.organisations.scope("o2");

        // then
        expect(startsWith(queryKeys.organisations.members("o1"), other)).toBe(false);
        expect(startsWith(queryKeys.organisations.list(), queryKeys.organisations.scope("list"))).toBe(false);
        expect(startsWith(queryKeys.organisations.list(), queryKeys.organisations.all)).toBe(true);
    });

    it("starts every key of one team with its scope, and keeps other teams out", () => {
        // given
        const scope = queryKeys.teams.scope("t1");

        // then
        expect(startsWith(queryKeys.teams.detail("t1"), scope)).toBe(true);
        expect(startsWith(queryKeys.teams.members("t1"), scope)).toBe(true);
        expect(startsWith(queryKeys.teams.members("t2"), scope)).toBe(false);
        expect(startsWith(scope, queryKeys.teams.all)).toBe(true);
    });

    it("keeps every live board key out of the boards root, so refreshing a board list never refetches an open board", () => {
        // given
        const live = [queryKeys.boardLive.all, queryKeys.boardLive.state("b1"), queryKeys.boardLive.mutations("b1")];

        // then
        for (const key of live) {
            expect(startsWith(key, queryKeys.boards.all)).toBe(false);
        }
    });

    it("keeps one board's state key out of another board's and out of its own mutation key", () => {
        // given
        const state = queryKeys.boardLive.state("b1");

        // then
        expect(startsWith(queryKeys.boardLive.state("b2"), state)).toBe(false);
        expect(startsWith(queryKeys.boardLive.mutations("b1"), state)).toBe(false);
        expect(startsWith(state, queryKeys.boardLive.all)).toBe(true);
        expect(startsWith(queryKeys.boards.team("t1"), queryKeys.boards.all)).toBe(true);
    });
});
