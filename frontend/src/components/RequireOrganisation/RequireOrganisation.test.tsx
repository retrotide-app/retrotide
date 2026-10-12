import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { PENDING_INVITE_STORAGE_KEY } from "../../platform/pendingInvite";
import { makeApiError, makeOrganisation, makeSessionUser } from "../../test-utils/fixtures";
import { type ProviderOptions, renderWithProviders } from "../../test-utils/render";
import { RequireOrganisation } from "./RequireOrganisation";

function renderAtHome(options: ProviderOptions) {
    return renderWithProviders(
        <Routes>
            <Route element={<RequireOrganisation />}>
                <Route path="/" element={<h1>Home</h1>} />
            </Route>
            <Route path="/onboarding" element={<h1>Onboarding</h1>} />
            <Route path="/invite" element={<h1>Invite</h1>} />
            <Route path="/admin" element={<h1>Admin</h1>} />
        </Routes>,
        { route: "/", ...options },
    );
}

describe("RequireOrganisation", () => {
    it("lets a member of an organisation through", () => {
        // when
        renderAtHome({ user: makeSessionUser(), org: { organisations: [makeOrganisation()] } });

        // then
        expect(screen.getByRole("heading", { name: "Home" })).toBeInTheDocument();
    });

    it("lets the owner of a deleted organisation through, so they can still restore it", () => {
        // given
        const deleted = makeOrganisation({ deleted_at: "2026-09-20T09:00:00Z" });

        // when
        renderAtHome({ user: makeSessionUser(), org: { organisations: [deleted] } });

        // then
        expect(screen.getByRole("heading", { name: "Home" })).toBeInTheDocument();
    });

    it("sends a signed in user with no organisation to onboarding", () => {
        // when
        renderAtHome({ user: makeSessionUser(), org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Onboarding" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Home" })).not.toBeInTheDocument();
    });

    it("sends site staff with no organisation to the admin dashboard, not to onboarding", () => {
        // when
        renderAtHome({
            user: makeSessionUser(),
            permissions: ["view_admin_panel", "view_users"],
            org: { organisations: [] },
        });

        // then
        expect(screen.getByRole("heading", { name: "Admin" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding" })).not.toBeInTheDocument();
    });

    it("lets site staff who belong to an organisation through to it", () => {
        // when
        renderAtHome({
            user: makeSessionUser(),
            permissions: ["view_admin_panel"],
            org: { organisations: [makeOrganisation()] },
        });

        // then
        expect(screen.getByRole("heading", { name: "Home" })).toBeInTheDocument();
    });

    it("sends site staff with a pending invite to the invite first", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        renderAtHome({ user: makeSessionUser(), permissions: ["view_admin_panel"], org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Invite" })).toBeInTheDocument();
    });

    it("sends a user with no organisation but a pending invite to the invite, not to onboarding", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        renderAtHome({ user: makeSessionUser(), org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Invite" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding" })).not.toBeInTheDocument();
    });

    it("waits for the organisations before sending anyone anywhere", () => {
        // when
        renderAtHome({ user: makeSessionUser(), org: { organisations: [], loading: true } });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Onboarding" })).not.toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Home" })).not.toBeInTheDocument();
    });

    it("shows why the organisations could not be read, instead of sending the user to onboarding", () => {
        // given
        const error = makeApiError({ status: 500, message: "failed to list your organisations" });

        // when
        renderAtHome({ user: makeSessionUser(), org: { organisations: [], error } });

        // then
        expect(screen.getByRole("heading", { name: "We could not load your organisations" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list your organisations");
        expect(screen.queryByRole("heading", { name: "Onboarding" })).not.toBeInTheDocument();
    });

    it("reads the organisations again when the user tries again", async () => {
        // given
        const retry = vi.fn().mockResolvedValue(undefined);
        const error = makeApiError({ status: 500, message: "failed to list your organisations" });
        const viewer = userEvent.setup();
        renderAtHome({ user: makeSessionUser(), org: { organisations: [], error, retry } });

        // when
        await viewer.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(retry).toHaveBeenCalledOnce();
    });

    it("lets a signed out visitor through, since nobody is there to need an organisation", () => {
        // when
        renderAtHome({ user: null, org: { organisations: [] } });

        // then
        expect(screen.getByRole("heading", { name: "Home" })).toBeInTheDocument();
    });

    it("guards the children it wraps as well as nested routes", () => {
        // when
        renderWithProviders(
            <Routes>
                <Route
                    path="/"
                    element={
                        <RequireOrganisation>
                            <h1>Home</h1>
                        </RequireOrganisation>
                    }
                />
                <Route path="/onboarding" element={<h1>Onboarding</h1>} />
            </Routes>,
            { user: makeSessionUser(), org: { organisations: [] } },
        );

        // then
        expect(screen.getByRole("heading", { name: "Onboarding" })).toBeInTheDocument();
    });
});
