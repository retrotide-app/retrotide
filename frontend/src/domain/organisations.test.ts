import { describe, expect, it } from "vitest";
import type { OrganisationResponse } from "../types/api";
import {
    chooseCurrentOrganisation,
    findOrganisationBySlug,
    findRenamedOrganisation,
    isArchived,
    isDeleted,
    isNoLongerListed,
    joinedOrganisationPath,
    organisationHomePath,
    organisationOptionLabel,
    organisationPath,
    renamedOrganisationPath,
    switchedOrganisationPath,
    teamPath,
    visibleOrganisationSections,
    withOrganisation,
} from "./organisations";

function organisation(overrides: Partial<OrganisationResponse> = {}): OrganisationResponse {
    return {
        id: "o1",
        slug: "harbour",
        name: "Harbour",
        allow_guests: true,
        role: "owner",
        created_at: "2026-09-01T09:00:00Z",
        ...overrides,
    };
}

const harbour = organisation();
const lighthouse = organisation({ id: "o2", slug: "lighthouse", name: "Lighthouse", role: "member" });
const sunk = organisation({ id: "o3", slug: "sunk", name: "Sunk", deleted_at: "2026-09-20T09:00:00Z" });

describe("isDeleted", () => {
    it("tells a deleted organisation from a live one", () => {
        // when
        const deleted = [isDeleted(harbour), isDeleted(sunk)];

        // then
        expect(deleted).toEqual([false, true]);
    });
});

describe("isArchived", () => {
    it("tells an archived team from an active one", () => {
        // given
        const team = {
            id: "t1",
            org_id: "o1",
            name: "Crew",
            created_at: "2026-09-01T09:00:00Z",
        };

        // when
        const archived = [isArchived(team), isArchived({ ...team, archived_at: "2026-09-20T09:00:00Z" })];

        // then
        expect(archived).toEqual([false, true]);
    });
});

describe("organisationOptionLabel", () => {
    it("names each organisation with the user's role in it", () => {
        // when
        const labels = [organisationOptionLabel(harbour), organisationOptionLabel(lighthouse)];

        // then
        expect(labels).toEqual(["Harbour (Owner)", "Lighthouse (Member)"]);
    });

    it("marks a deleted organisation as deleted", () => {
        // when
        const label = organisationOptionLabel(sunk);

        // then
        expect(label).toBe("Sunk (deleted)");
    });

    it("names an organisation listed without a role by its name alone", () => {
        // when
        const label = organisationOptionLabel(organisation({ role: undefined }));

        // then
        expect(label).toBe("Harbour");
    });
});

describe("chooseCurrentOrganisation", () => {
    it("keeps the organisation chosen in this browser", () => {
        // when
        const current = chooseCurrentOrganisation([harbour, lighthouse], "o2");

        // then
        expect(current).toBe(lighthouse);
    });

    it("keeps a chosen organisation even once it is deleted, so its owner can restore it", () => {
        // when
        const current = chooseCurrentOrganisation([harbour, sunk], "o3");

        // then
        expect(current).toBe(sunk);
    });

    it("falls back to the first live organisation when nothing was chosen", () => {
        // when
        const current = chooseCurrentOrganisation([sunk, lighthouse, harbour], null);

        // then
        expect(current).toBe(lighthouse);
    });

    it("falls back to the first live organisation when the chosen one is no longer listed", () => {
        // when
        const current = chooseCurrentOrganisation([harbour, lighthouse], "gone");

        // then
        expect(current).toBe(harbour);
    });

    it("falls back to a deleted organisation when no live one is left", () => {
        // when
        const current = chooseCurrentOrganisation([sunk], null);

        // then
        expect(current).toBe(sunk);
    });

    it("finds none when the user belongs to no organisation", () => {
        // when
        const current = chooseCurrentOrganisation([], "o1");

        // then
        expect(current).toBeNull();
    });
});

describe("findOrganisationBySlug", () => {
    it("finds the organisation an address names", () => {
        // when
        const found = [findOrganisationBySlug([harbour, lighthouse], "lighthouse"), findOrganisationBySlug([], "x")];

        // then
        expect(found).toEqual([lighthouse, null]);
    });
});

describe("findRenamedOrganisation", () => {
    it("finds the organisation last seen at an address that it has since left for another", () => {
        // given
        const renamed = organisation({ slug: "harbour-crew" });

        // when
        const found = findRenamedOrganisation([renamed, lighthouse], { id: "o1", slug: "harbour" }, "harbour");

        // then
        expect(found).toBe(renamed);
    });

    it("finds nothing for an address that was never seen here, so a stranger's address is not followed", () => {
        // when
        const found = findRenamedOrganisation([harbour], { id: "o1", slug: "harbour" }, "elsewhere");

        // then
        expect(found).toBeNull();
    });

    it("finds nothing once the organisation seen there is no longer listed", () => {
        // when
        const found = findRenamedOrganisation([lighthouse], { id: "o1", slug: "harbour" }, "harbour");

        // then
        expect(found).toBeNull();
    });

    it("finds nothing when nothing was seen yet, or the organisation kept its address", () => {
        // when
        const found = [
            findRenamedOrganisation([harbour], null, "harbour"),
            findRenamedOrganisation([harbour], { id: "o1", slug: "harbour" }, "harbour"),
        ];

        // then
        expect(found).toEqual([null, null]);
    });
});

describe("isNoLongerListed", () => {
    it("tells when the organisation last seen at an address has left the listing", () => {
        // when
        const gone = isNoLongerListed([lighthouse], { id: "o1", slug: "harbour" }, "harbour");

        // then
        expect(gone).toBe(true);
    });

    it("keeps an organisation that is still listed, even under another address", () => {
        // when
        const gone = [
            isNoLongerListed([harbour, lighthouse], { id: "o1", slug: "harbour" }, "harbour"),
            isNoLongerListed([organisation({ slug: "harbour-crew" })], { id: "o1", slug: "harbour" }, "harbour"),
        ];

        // then
        expect(gone).toEqual([false, false]);
    });

    it("says nothing about an address never seen here, so a stranger's address stays not found", () => {
        // when
        const gone = [
            isNoLongerListed([lighthouse], null, "harbour"),
            isNoLongerListed([lighthouse], { id: "o1", slug: "harbour" }, "elsewhere"),
        ];

        // then
        expect(gone).toEqual([false, false]);
    });
});

describe("renamedOrganisationPath", () => {
    it("puts the new address in place of the old and keeps the rest of the path", () => {
        // when
        const paths = [
            renamedOrganisationPath("/o/harbour/settings", "harbour-crew"),
            renamedOrganisationPath("/o/harbour/teams/t1", "harbour-crew"),
        ];

        // then
        expect(paths).toEqual(["/o/harbour-crew/settings", "/o/harbour-crew/teams/t1"]);
    });
});

describe("organisation paths", () => {
    it("builds each section's path under the organisation's address", () => {
        // when
        const paths = [
            organisationPath("harbour"),
            organisationPath("harbour", "members"),
            organisationPath("harbour", "invites"),
            organisationPath("harbour", "settings"),
        ];

        // then
        expect(paths).toEqual(["/o/harbour/teams", "/o/harbour/members", "/o/harbour/invites", "/o/harbour/settings"]);
    });

    it("builds a team's path under its organisation's teams", () => {
        // when
        const path = teamPath("harbour", "t1");

        // then
        expect(path).toBe("/o/harbour/teams/t1");
    });

    it("builds the organisation's home at its bare address, escaped", () => {
        // when
        const paths = [organisationHomePath("harbour"), organisationHomePath("a/b")];

        // then
        expect(paths).toEqual(["/o/harbour", "/o/a%2Fb"]);
    });
});

describe("joinedOrganisationPath", () => {
    it("leads to the team an invite joined", () => {
        // given
        const joined = { org_slug: "harbour", team_id: "t1" };

        // when
        const path = joinedOrganisationPath(joined);

        // then
        expect(path).toBe("/o/harbour/teams/t1");
    });

    it("leads to the organisation's home when the invite joined no team", () => {
        // given
        const joined = { org_slug: "harbour", team_id: undefined };

        // when
        const path = joinedOrganisationPath(joined);

        // then
        expect(path).toBe("/o/harbour");
    });
});

describe("switchedOrganisationPath", () => {
    it("keeps the section when switching on an organisation page", () => {
        // when
        const path = switchedOrganisationPath("/o/harbour/members", "lighthouse");

        // then
        expect(path).toBe("/o/lighthouse/members");
    });

    it("keeps the action items when switching on that page", () => {
        // when
        const path = switchedOrganisationPath("/o/harbour/action-items", "lighthouse");

        // then
        expect(path).toBe("/o/lighthouse/action-items");
    });

    it("goes to the other organisation's teams from a team page, because the team is not theirs", () => {
        // when
        const path = switchedOrganisationPath("/o/harbour/teams/t1", "lighthouse");

        // then
        expect(path).toBe("/o/lighthouse/teams");
    });

    it("goes to the other organisation's home from a section it does not know", () => {
        // when
        const path = switchedOrganisationPath("/o/harbour/billing", "lighthouse");

        // then
        expect(path).toBe("/o/lighthouse");
    });

    it("goes to the other organisation's home from a board, because the board is not theirs", () => {
        // when
        const paths = [
            switchedOrganisationPath("/o/harbour/boards/b1", "lighthouse"),
            switchedOrganisationPath("/o/harbour/boards/new", "lighthouse"),
        ];

        // then
        expect(paths).toEqual(["/o/lighthouse", "/o/lighthouse"]);
    });

    it("goes to the other organisation's home from this one's home", () => {
        // when
        const paths = [
            switchedOrganisationPath("/o/harbour", "lighthouse"),
            switchedOrganisationPath("/o/harbour/", "lighthouse"),
        ];

        // then
        expect(paths).toEqual(["/o/lighthouse", "/o/lighthouse"]);
    });

    it("stays put away from the organisation pages", () => {
        // when
        const paths = [switchedOrganisationPath("/account", "lighthouse"), switchedOrganisationPath("/", "x")];

        // then
        expect(paths).toEqual([null, null]);
    });
});

describe("visibleOrganisationSections", () => {
    it("shows the owner and admins every section", () => {
        // when
        const sections = [
            visibleOrganisationSections(harbour),
            visibleOrganisationSections(organisation({ role: "admin" })),
        ];

        // then
        expect(sections).toEqual([
            ["teams", "action-items", "templates", "members", "invites", "settings"],
            ["teams", "action-items", "templates", "members", "invites", "settings"],
        ]);
    });

    it("shows a member the teams, the action items and the templates only", () => {
        // when
        const sections = visibleOrganisationSections(lighthouse);

        // then
        expect(sections).toEqual(["teams", "action-items", "templates"]);
    });

    it("shows the owner of a deleted organisation only its settings, where it is restored", () => {
        // when
        const sections = [visibleOrganisationSections(sunk), visibleOrganisationSections({ ...sunk, role: "admin" })];

        // then
        expect(sections).toEqual([["settings"], []]);
    });

    it("shows nothing without an organisation", () => {
        // when
        const sections = visibleOrganisationSections(null);

        // then
        expect(sections).toEqual([]);
    });
});

describe("withOrganisation", () => {
    it("adds a new organisation to the listing", () => {
        // when
        const listing = withOrganisation({ organisations: [lighthouse] }, harbour);

        // then
        expect(listing).toEqual({ organisations: [lighthouse, harbour] });
    });

    it("replaces the listed copy of the same organisation", () => {
        // given
        const renamed = organisation({ name: "Harbour Crew" });

        // when
        const listing = withOrganisation({ organisations: [harbour, lighthouse] }, renamed);

        // then
        expect(listing).toEqual({ organisations: [lighthouse, renamed] });
    });

    it("starts a listing that was never read", () => {
        // when
        const listing = withOrganisation(undefined, harbour);

        // then
        expect(listing).toEqual({ organisations: [harbour] });
    });
});
