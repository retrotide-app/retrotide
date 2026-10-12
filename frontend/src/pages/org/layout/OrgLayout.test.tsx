import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { OrgContext } from "../../../context/orgContextValue";
import { makeOrganisation, makeOrgContext } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { OrganisationResponse } from "../../../types/api";
import { OrgLayout } from "./OrgLayout";

const harbour = makeOrganisation({ id: "o1", slug: "harbour", name: "Harbour" });
const lighthouse = makeOrganisation({ id: "o2", slug: "lighthouse", name: "Lighthouse", role: "member" });

function renderLayout(route: string, current: OrganisationResponse | null, selectOrganisation = vi.fn()) {
    renderWithProviders(
        <Routes>
            <Route path="/o/:orgSlug" element={<OrgLayout />}>
                <Route path="teams" element={<h1>Teams page</h1>} />
            </Route>
        </Routes>,
        { route, org: { organisations: [harbour, lighthouse], current, selectOrganisation } },
    );

    return selectOrganisation;
}

function SettingsProbe() {
    const [saved, setSaved] = useState(false);
    const location = useLocation();

    return (
        <section>
            <h1>Settings page</h1>
            <p>{`at ${location.pathname}`}</p>
            <button type="button" onClick={() => setSaved(true)}>
                Save
            </button>
            {saved && <p>Saved</p>}
        </section>
    );
}

interface ListedProps {
    organisation: OrganisationResponse;
}

function Listed({ organisation }: ListedProps) {
    return (
        <OrgContext.Provider value={makeOrgContext({ organisations: [organisation], current: organisation })}>
            <Routes>
                <Route path="/o/:orgSlug" element={<OrgLayout />}>
                    <Route path="settings" element={<SettingsProbe />} />
                </Route>
            </Routes>
        </OrgContext.Provider>
    );
}

function Landed() {
    const location = useLocation();

    return <p>{`landed on ${location.pathname}`}</p>;
}

interface MembershipProps {
    organisations: OrganisationResponse[];
}

function Membership({ organisations }: MembershipProps) {
    return (
        <OrgContext.Provider value={makeOrgContext({ organisations, current: organisations[0] ?? null })}>
            <Routes>
                <Route path="/o/:orgSlug" element={<OrgLayout />}>
                    <Route path="members" element={<h1>Members page</h1>} />
                    <Route index element={<Landed />} />
                </Route>
            </Routes>
        </OrgContext.Provider>
    );
}

describe("OrgLayout", () => {
    it("draws the page when the address names the current organisation", () => {
        // when
        const selectOrganisation = renderLayout("/o/harbour/teams", harbour);

        // then
        expect(screen.getByRole("heading", { name: "Teams page" })).toBeInTheDocument();
        expect(selectOrganisation).not.toHaveBeenCalled();
    });

    it("makes the organisation the address names current, and holds the page until it is", () => {
        // when
        const selectOrganisation = renderLayout("/o/lighthouse/teams", harbour);

        // then
        expect(selectOrganisation).toHaveBeenCalledExactlyOnceWith("o2");
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Teams page" })).not.toBeInTheDocument();
    });

    it("says the organisation was not found when the user is not a member of it", () => {
        // when
        const selectOrganisation = renderLayout("/o/elsewhere/teams", harbour);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Organisation not found" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Go to the home page" })).toHaveAttribute("href", "/");
        expect(screen.queryByRole("heading", { name: "Teams page" })).not.toBeInTheDocument();
        expect(selectOrganisation).not.toHaveBeenCalled();
    });

    it("titles the page when the organisation was not found", () => {
        // when
        renderLayout("/o/elsewhere/teams", harbour);

        // then
        expect(document.title).toContain("Organisation not found");
    });

    it("follows the organisation to its new address, keeping the page it was on as it was", async () => {
        // given
        const viewer = userEvent.setup();
        const { rerender } = renderWithProviders(<Listed organisation={harbour} />, { route: "/o/harbour/settings" });
        await viewer.click(screen.getByRole("button", { name: "Save" }));

        // when
        rerender(<Listed organisation={{ ...harbour, slug: "harbour-crew" }} />);

        // then
        expect(await screen.findByText("at /o/harbour-crew/settings")).toBeInTheDocument();
        expect(screen.getByText("Saved")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation not found" })).not.toBeInTheDocument();
    });

    it("moves to the home of the organisation that is now current once the one on the page leaves the listing", async () => {
        // given
        const { rerender } = renderWithProviders(<Membership organisations={[harbour, lighthouse]} />, {
            route: "/o/harbour/members",
        });

        // when
        rerender(<Membership organisations={[lighthouse]} />);

        // then
        expect(await screen.findByText("landed on /o/lighthouse")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Organisation not found" })).not.toBeInTheDocument();
    });
});
