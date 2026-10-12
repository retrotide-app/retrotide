import type {
    JoinedOrganisationResponse,
    OrganisationListResponse,
    OrganisationResponse,
    TeamResponse,
} from "../types/api";
import { ORG_ROLE_LABELS, orgRoleGrants } from "./organisationRoles";

export type OrganisationSection = "teams" | "action-items" | "templates" | "members" | "invites" | "settings";

export interface SeenOrganisation {
    id: string;
    slug: string;
}

export const ORGANISATION_NAME_MAX_LENGTH = 80;

export const TEAM_NAME_MAX_LENGTH = 80;

export const ONBOARDING_PATH = "/onboarding";

export const INVITE_PATH = "/invite";

export const ORGANISATION_SECTION_LABELS: Readonly<Record<OrganisationSection, string>> = {
    teams: "Teams",
    "action-items": "Action items",
    templates: "Templates",
    members: "Members",
    invites: "Invites",
    settings: "Organisation settings",
};

const ORGANISATION_SECTIONS: readonly OrganisationSection[] = [
    "teams",
    "action-items",
    "templates",
    "members",
    "invites",
    "settings",
];

const ORGANISATION_ROUTE = /^\/o\/[^/]+\/([^/]+)/;

const ORGANISATION_ADDRESS = /^\/o\/[^/]+/;

export function isDeleted(organisation: OrganisationResponse): boolean {
    return organisation.deleted_at !== undefined && organisation.deleted_at !== "";
}

export function isArchived(team: TeamResponse): boolean {
    return team.archived_at !== undefined && team.archived_at !== "";
}

export function organisationOptionLabel(organisation: OrganisationResponse): string {
    if (isDeleted(organisation)) {
        return `${organisation.name} (deleted)`;
    }

    if (organisation.role === undefined) {
        return organisation.name;
    }

    return `${organisation.name} (${ORG_ROLE_LABELS[organisation.role]})`;
}

export function chooseCurrentOrganisation(
    organisations: readonly OrganisationResponse[],
    chosenId: string | null,
): OrganisationResponse | null {
    const chosen = organisations.find(organisation => organisation.id === chosenId);

    if (chosen !== undefined) {
        return chosen;
    }

    return organisations.find(organisation => !isDeleted(organisation)) ?? organisations[0] ?? null;
}

export function findOrganisationBySlug(
    organisations: readonly OrganisationResponse[],
    slug: string,
): OrganisationResponse | null {
    return organisations.find(organisation => organisation.slug === slug) ?? null;
}

export function findRenamedOrganisation(
    organisations: readonly OrganisationResponse[],
    seen: SeenOrganisation | null,
    slug: string,
): OrganisationResponse | null {
    if (seen === null || seen.slug !== slug) {
        return null;
    }

    const found = organisations.find(organisation => organisation.id === seen.id);

    if (found === undefined || found.slug === slug) {
        return null;
    }

    return found;
}

export function isNoLongerListed(
    organisations: readonly OrganisationResponse[],
    seen: SeenOrganisation | null,
    slug: string,
): boolean {
    if (seen === null || seen.slug !== slug) {
        return false;
    }

    return !organisations.some(organisation => organisation.id === seen.id);
}

export function renamedOrganisationPath(pathname: string, slug: string): string {
    return pathname.replace(ORGANISATION_ADDRESS, `/o/${encodeURIComponent(slug)}`);
}

export function organisationHomePath(slug: string): string {
    return `/o/${encodeURIComponent(slug)}`;
}

export function organisationPath(slug: string, section: OrganisationSection = "teams"): string {
    return `${organisationHomePath(slug)}/${section}`;
}

export function teamPath(slug: string, teamId: string): string {
    return `${organisationPath(slug, "teams")}/${encodeURIComponent(teamId)}`;
}

export function joinedOrganisationPath(joined: Pick<JoinedOrganisationResponse, "org_slug" | "team_id">): string {
    if (joined.team_id === undefined) {
        return organisationHomePath(joined.org_slug);
    }

    return teamPath(joined.org_slug, joined.team_id);
}

function isOrganisationSection(value: string): value is OrganisationSection {
    return ORGANISATION_SECTIONS.some(section => section === value);
}

export function switchedOrganisationPath(pathname: string, slug: string): string | null {
    if (!ORGANISATION_ADDRESS.test(pathname)) {
        return null;
    }

    const section = ORGANISATION_ROUTE.exec(pathname)?.[1] ?? "";

    if (isOrganisationSection(section)) {
        return organisationPath(slug, section);
    }

    return organisationHomePath(slug);
}

export function visibleOrganisationSections(organisation: OrganisationResponse | null): OrganisationSection[] {
    if (organisation === null) {
        return [];
    }

    const { role } = organisation;

    if (isDeleted(organisation)) {
        return orgRoleGrants(role, "org.delete") ? ["settings"] : [];
    }

    return ORGANISATION_SECTIONS.filter(section => {
        if (section === "invites" || section === "members") {
            return orgRoleGrants(role, "org.manage_members");
        }

        if (section === "settings") {
            return orgRoleGrants(role, "org.update");
        }

        return true;
    });
}

export function withOrganisation(
    listing: OrganisationListResponse | undefined,
    organisation: OrganisationResponse,
): OrganisationListResponse {
    const others = (listing?.organisations ?? []).filter(listed => listed.id !== organisation.id);

    return { organisations: [...others, organisation] };
}
