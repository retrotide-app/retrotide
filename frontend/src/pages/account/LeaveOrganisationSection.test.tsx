import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeMutationState, makeOrganisation, makeSessionUser } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { OrganisationResponse } from "../../types/api";
import { LeaveOrganisationSection } from "./LeaveOrganisationSection";

const { useLeaveOrganisation } = vi.hoisted(() => ({ useLeaveOrganisation: vi.fn() }));

vi.mock("../../hooks/mutations/organisations", () => ({ useLeaveOrganisation }));

const harbour = makeOrganisation({ id: "org-harbour", name: "Harbour", role: "member" });
const docks = makeOrganisation({ id: "org-docks", name: "Docks", role: "owner" });

function renderSection(organisations: OrganisationResponse[]) {
    return renderWithProviders(<LeaveOrganisationSection />, {
        user: makeSessionUser(),
        org: { organisations, current: organisations[0] ?? null },
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    useLeaveOrganisation.mockReturnValue(makeMutationState());
});

describe("LeaveOrganisationSection", () => {
    it("offers to leave every organisation the viewer may leave, never one they own", () => {
        // when
        renderSection([harbour, docks]);

        // then
        const list = screen.getByRole("list");
        expect(within(list).getByText("Harbour")).toBeInTheDocument();
        expect(within(list).queryByText("Docks")).not.toBeInTheDocument();
    });

    it("shows nothing when the viewer owns every organisation they are in", () => {
        // when
        renderSection([docks]);

        // then
        expect(screen.queryByRole("heading", { name: "Organisations" })).not.toBeInTheDocument();
    });

    it("leaves the organisation once confirmed", async () => {
        // given
        const leave = makeMutationState();
        useLeaveOrganisation.mockReturnValue(leave);
        const user = userEvent.setup();
        renderSection([harbour]);

        // when
        await user.click(screen.getByRole("button", { name: "Leave" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Leave Harbour?" })).getByRole("button", {
                name: "Leave organisation",
            }),
        );

        // then
        expect(leave.mutate).toHaveBeenCalledWith({ orgId: "org-harbour" }, expect.anything());
    });
});
