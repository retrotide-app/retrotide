import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { makeApiError, makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { OrgSwitcher } from "./OrgSwitcher";

const harbour = makeOrganisation({ id: "o1", slug: "harbour", name: "Harbour", role: "owner" });
const lighthouse = makeOrganisation({ id: "o2", slug: "lighthouse", name: "Lighthouse", role: "member" });
const sunk = makeOrganisation({ id: "o3", slug: "sunk", name: "Sunk", deleted_at: "2026-09-20T09:00:00Z" });

function Where() {
    const location = useLocation();

    return <p>{`at ${location.pathname}`}</p>;
}

function renderSwitcher(route: string, selectOrganisation = vi.fn()) {
    renderWithProviders(
        <>
            <OrgSwitcher />
            <Routes>
                <Route path="*" element={<Where />} />
            </Routes>
        </>,
        { route, org: { organisations: [harbour, lighthouse, sunk], current: harbour, selectOrganisation } },
    );

    return selectOrganisation;
}

describe("OrgSwitcher", () => {
    it("lists the user's organisations with their role, and marks a deleted one", () => {
        // when
        renderSwitcher("/");

        // then
        const options = screen.getAllByRole("option").map(option => option.textContent);
        expect(options).toEqual(["Harbour (Owner)", "Lighthouse (Member)", "Sunk (deleted)"]);
    });

    it("shows the current organisation as chosen, under a visible label", () => {
        // when
        renderSwitcher("/");

        // then
        expect(screen.getByRole("combobox", { name: "Organisation" })).toHaveValue("o1");
    });

    it("switches the current organisation and stays on a page outside the organisation", async () => {
        // given
        const viewer = userEvent.setup();
        const selectOrganisation = renderSwitcher("/account");

        // when
        await viewer.selectOptions(screen.getByRole("combobox", { name: "Organisation" }), "Lighthouse (Member)");

        // then
        expect(selectOrganisation).toHaveBeenCalledExactlyOnceWith("o2");
        expect(screen.getByText("at /account")).toBeInTheDocument();
    });

    it("moves to the same section of the chosen organisation from an organisation page", async () => {
        // given
        const viewer = userEvent.setup();
        renderSwitcher("/o/harbour/members");

        // when
        await viewer.selectOptions(screen.getByRole("combobox", { name: "Organisation" }), "Lighthouse (Member)");

        // then
        expect(screen.getByText("at /o/lighthouse/members")).toBeInTheDocument();
    });

    it("draws nothing for a user with no organisation", () => {
        // when
        renderWithProviders(<OrgSwitcher />, { org: { organisations: [] } });

        // then
        expect(screen.queryByRole("combobox", { name: "Organisation" })).not.toBeInTheDocument();
    });

    it("shows why the organisations could not be read, and reads them again on request", async () => {
        // given
        const retry = vi.fn().mockResolvedValue(undefined);
        const error = makeApiError({ status: 500, message: "failed to list your organisations" });
        const viewer = userEvent.setup();
        renderWithProviders(<OrgSwitcher />, { org: { organisations: [], error, retry } });

        // when
        await viewer.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list your organisations");
        expect(retry).toHaveBeenCalledOnce();
    });
});
