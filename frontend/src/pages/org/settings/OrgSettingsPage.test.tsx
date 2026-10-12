import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { OrgContext } from "../../../context/orgContextValue";
import { makeMutationState, makeOrganisation, makeOrgContext, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { OrganisationResponse, OrgRole } from "../../../types/api";
import { OrgSettingsPage } from "./OrgSettingsPage";

const { useUpdateOrganisation, useDeleteOrganisation, useRestoreOrganisation } = vi.hoisted(() => ({
    useUpdateOrganisation: vi.fn(),
    useDeleteOrganisation: vi.fn(),
    useRestoreOrganisation: vi.fn(),
}));

vi.mock("../../../hooks/mutations/organisations", () => ({
    useUpdateOrganisation,
    useDeleteOrganisation,
    useRestoreOrganisation,
}));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });
const deletedAt = "2026-09-20T12:00:00Z";
const restorableUntil = "2026-10-20T12:00:00Z";

function renderPage(role: OrgRole, deleted_at?: string, restorable_until?: string) {
    const current = makeOrganisation({ ...harbour, role, deleted_at, restorable_until });

    return renderWithProviders(<OrgSettingsPage />, {
        user: makeSessionUser(),
        org: { organisations: [current], current },
    });
}

interface ListedProps {
    organisation: OrganisationResponse;
}

function Listed({ organisation }: ListedProps) {
    return (
        <OrgContext.Provider value={makeOrgContext({ organisations: [organisation], current: organisation })}>
            <OrgSettingsPage />
        </OrgContext.Provider>
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    useUpdateOrganisation.mockReturnValue(makeMutationState());
    useDeleteOrganisation.mockReturnValue(makeMutationState());
    useRestoreOrganisation.mockReturnValue(makeMutationState());
});

describe("OrgSettingsPage", () => {
    it("lets the owner rename the organisation, change its address and guests, and delete it", () => {
        // when
        renderPage("owner");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Organisation settings" })).toBeInTheDocument();
        expect(document.title).toContain("Organisation settings");
        expect(screen.getByRole("button", { name: "Rename" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Change address" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Allow guests" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Delete organisation" })).toBeInTheDocument();
    });

    it("lets an admin change the settings but not delete the organisation", () => {
        // when
        renderPage("admin");

        // then
        expect(screen.getByRole("button", { name: "Rename" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Change address" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Allow guests" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Delete organisation" })).not.toBeInTheDocument();
    });

    it.each(["owner", "admin"] as const)("lets the %s download the organisation's export by its ID", role => {
        // when
        renderPage(role);

        // then
        const section = within(screen.getByRole("region", { name: "Export organisation" }));
        const link = section.getByRole("link", { name: "Download export" });
        expect(link).toHaveAttribute("href", "/api/v1/orgs/org-harbour/export");
        expect(link).toHaveAttribute("download");
    });

    it("offers a plain member no organisation export", () => {
        // when
        renderPage("member");

        // then
        expect(screen.queryByRole("region", { name: "Export organisation" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Download export" })).not.toBeInTheDocument();
    });

    it("tells a member only the owner and admins change the settings", () => {
        // when
        renderPage("member");

        // then
        expect(screen.getByText("Only the owner and admins can change these settings.")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Rename" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Delete organisation" })).not.toBeInTheDocument();
    });

    it("offers the owner of a deleted organisation only the way to restore it, until the server's deadline", () => {
        // when
        renderPage("owner", deletedAt, restorableUntil);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Organisation settings" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Restore organisation" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Restore organisation" })).toHaveTextContent(
            /You can restore it until 20 October 2026/,
        );
        expect(screen.queryByRole("button", { name: "Rename" })).not.toBeInTheDocument();
        expect(screen.queryByRole("switch", { name: "Allow guests" })).not.toBeInTheDocument();
    });

    it("keeps saying the organisation was deleted once the page turns to restoring it", () => {
        // given
        useDeleteOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));
        const { rerender } = renderWithProviders(<Listed organisation={harbour} />, { user: makeSessionUser() });

        // when
        rerender(<Listed organisation={makeOrganisation({ ...harbour, deleted_at: deletedAt })} />);

        // then
        expect(screen.getByRole("button", { name: "Restore organisation" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Harbour deleted.");
    });
});
