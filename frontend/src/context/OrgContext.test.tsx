import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useCurrentOrg } from "../hooks/useCurrentOrg";
import { makeApiError, makeOrganisation, makeQueryState, makeSessionUser } from "../test-utils/fixtures";
import { renderWithProviders } from "../test-utils/render";
import type { OrganisationListResponse } from "../types/api";
import type { SessionUser } from "../types/app";
import { OrgProvider } from "./OrgContext";

const { useOrganisations } = vi.hoisted(() => ({ useOrganisations: vi.fn() }));

const { readLastOrganisation, writeLastOrganisation } = vi.hoisted(() => ({
    readLastOrganisation: vi.fn<() => string | null>(),
    writeLastOrganisation: vi.fn<(orgId: string) => void>(),
}));

vi.mock("../hooks/queries/organisations", () => ({ useOrganisations }));

vi.mock("../platform/lastOrganisation", () => ({ readLastOrganisation, writeLastOrganisation }));

const harbour = makeOrganisation({ id: "o1", name: "Harbour", role: "owner" });
const lighthouse = makeOrganisation({ id: "o2", slug: "lighthouse", name: "Lighthouse", role: "member" });

function Probe() {
    const { organisations, current, role, loading, error, selectOrganisation } = useCurrentOrg();

    return (
        <div>
            <p>{`organisations: ${organisations.map(listed => listed.name).join(", ") || "none"}`}</p>
            <p>{`current: ${current?.name ?? "none"}`}</p>
            <p>{`role: ${role ?? "none"}`}</p>
            <p>{`loading: ${String(loading)}`}</p>
            <p>{`error: ${error?.message ?? "none"}`}</p>
            <button type="button" onClick={() => selectOrganisation("o2")}>
                choose Lighthouse
            </button>
        </div>
    );
}

function renderProvider(user: SessionUser | null = makeSessionUser()) {
    return renderWithProviders(
        <OrgProvider>
            <Probe />
        </OrgProvider>,
        { user, org: { organisations: [makeOrganisation({ name: "Outer Harness" })] } },
    );
}

function listing(organisations: OrganisationListResponse["organisations"]) {
    return makeQueryState<OrganisationListResponse>({ data: { organisations } });
}

beforeEach(() => {
    readLastOrganisation.mockReset();
    readLastOrganisation.mockReturnValue(null);
    writeLastOrganisation.mockReset();
    useOrganisations.mockReturnValue(listing([harbour, lighthouse]));
});

describe("OrgProvider for a signed in user", () => {
    it("lists the user's organisations and makes the first one current when none was chosen", () => {
        // when
        renderProvider();

        // then
        expect(screen.getByText("organisations: Harbour, Lighthouse")).toBeInTheDocument();
        expect(screen.getByText("current: Harbour")).toBeInTheDocument();
        expect(screen.getByText("role: owner")).toBeInTheDocument();
    });

    it("asks for the organisations only while someone is signed in", () => {
        // when
        renderProvider();

        // then
        expect(useOrganisations).toHaveBeenCalledWith(true);
    });

    it("comes back to the organisation this browser chose last, with the user's role in it", () => {
        // given
        readLastOrganisation.mockReturnValue("o2");

        // when
        renderProvider();

        // then
        expect(screen.getByText("current: Lighthouse")).toBeInTheDocument();
        expect(screen.getByText("role: member")).toBeInTheDocument();
    });

    it("falls back to the first organisation when the one chosen last is no longer listed", () => {
        // given
        readLastOrganisation.mockReturnValue("gone");

        // when
        renderProvider();

        // then
        expect(screen.getByText("current: Harbour")).toBeInTheDocument();
    });

    it("switches to the organisation chosen and remembers it for this browser", async () => {
        // given
        const viewer = userEvent.setup();
        renderProvider();

        // when
        await viewer.click(screen.getByRole("button", { name: "choose Lighthouse" }));

        // then
        expect(screen.getByText("current: Lighthouse")).toBeInTheDocument();
        expect(screen.getByText("role: member")).toBeInTheDocument();
        expect(writeLastOrganisation).toHaveBeenCalledExactlyOnceWith("o2");
    });

    it("reports loading while the organisations are read", () => {
        // given
        useOrganisations.mockReturnValue(makeQueryState({ loading: true }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("loading: true")).toBeInTheDocument();
        expect(screen.getByText("current: none")).toBeInTheDocument();
    });

    it("reports the error when the organisations could not be read, rather than claiming there are none", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to list your organisations" });
        useOrganisations.mockReturnValue(makeQueryState({ error: failure }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("error: failed to list your organisations")).toBeInTheDocument();
    });

    it("has no current organisation for a user who belongs to none", () => {
        // given
        useOrganisations.mockReturnValue(listing([]));

        // when
        renderProvider();

        // then
        expect(screen.getByText("organisations: none")).toBeInTheDocument();
        expect(screen.getByText("current: none")).toBeInTheDocument();
        expect(screen.getByText("role: none")).toBeInTheDocument();
    });
});

describe("OrgProvider for a signed out visitor", () => {
    it("asks for nothing and lists no organisation, whatever the cache still holds", () => {
        // given
        useOrganisations.mockReturnValue(listing([harbour]));

        // when
        renderProvider(null);

        // then
        expect(useOrganisations).toHaveBeenCalledWith(false);
        expect(screen.getByText("organisations: none")).toBeInTheDocument();
        expect(screen.getByText("loading: false")).toBeInTheDocument();
    });
});
