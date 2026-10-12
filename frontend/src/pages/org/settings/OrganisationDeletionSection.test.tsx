import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { OrganisationDeletionSection } from "./OrganisationDeletionSection";

const { useDeleteOrganisation, useRestoreOrganisation } = vi.hoisted(() => ({
    useDeleteOrganisation: vi.fn(),
    useRestoreOrganisation: vi.fn(),
}));

vi.mock("../../../hooks/mutations/organisations", () => ({ useDeleteOrganisation, useRestoreOrganisation }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });
const deletedHarbour = makeOrganisation({
    ...harbour,
    deleted_at: "2026-09-20T12:00:00Z",
    restorable_until: "2026-10-20T12:00:00Z",
});

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    vi.clearAllMocks();
    useDeleteOrganisation.mockReturnValue(makeMutationState());
    useRestoreOrganisation.mockReturnValue(makeMutationState());
});

describe("OrganisationDeletionSection deleting", () => {
    it("says what deleting does and that it can be undone for a while", () => {
        // when
        renderWithProviders(<OrganisationDeletionSection organisation={harbour} />);

        // then
        expect(screen.getByRole("region", { name: "Delete organisation" })).toHaveTextContent(
            "Deleting hides Harbour and its teams from every member. You can restore it for a limited time.",
        );
        expect(screen.queryByRole("button", { name: "Restore organisation" })).not.toBeInTheDocument();
    });

    it("asks before deleting the organisation, then deletes it and says so", async () => {
        // given
        const deletion = makeMutationState({
            mutate: vi.fn((variables: unknown, options?: { onSuccess?: () => void }) => {
                useDeleteOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));
                succeed(variables, options);
            }),
        });
        useDeleteOrganisation.mockReturnValue(deletion);
        const user = userEvent.setup();
        renderWithProviders(<OrganisationDeletionSection organisation={harbour} />);

        // when
        await user.click(screen.getByRole("button", { name: "Delete organisation" }));
        const dialog = screen.getByRole("dialog", { name: "Delete Harbour?" });
        const body = dialog.textContent;
        await user.click(within(dialog).getByRole("button", { name: "Delete organisation" }));

        // then
        expect(body).toContain(
            "Every member loses access to Harbour and its teams. You can restore it from this page for a limited time.",
        );
        expect(deletion.mutate).toHaveBeenCalledExactlyOnceWith({ orgId: "org-harbour" }, expect.anything());
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Harbour deleted.");
    });

    it("keeps the organisation when the owner changes their mind", async () => {
        // given
        const deletion = makeMutationState();
        useDeleteOrganisation.mockReturnValue(deletion);
        const user = userEvent.setup();
        renderWithProviders(<OrganisationDeletionSection organisation={harbour} />);

        // when
        await user.click(screen.getByRole("button", { name: "Delete organisation" }));
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(deletion.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("keeps the check open with the server's reason when deleting fails", async () => {
        // given
        const error = makeApiError({ status: 404, message: "organisation not found" });
        useDeleteOrganisation.mockReturnValue(makeMutationState({ isError: true, error }));
        const user = userEvent.setup();
        renderWithProviders(<OrganisationDeletionSection organisation={harbour} />);

        // when
        await user.click(screen.getByRole("button", { name: "Delete organisation" }));

        // then
        expect(screen.getByRole("dialog", { name: "Delete Harbour?" })).toHaveTextContent("organisation not found");
    });

    it("turns to restoring once the organisation is deleted, and still says it was deleted", () => {
        // given
        useDeleteOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));
        const { rerender } = renderWithProviders(<OrganisationDeletionSection organisation={harbour} />);

        // when
        rerender(<OrganisationDeletionSection organisation={deletedHarbour} />);

        // then
        expect(screen.getByRole("button", { name: "Restore organisation" })).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Harbour deleted.");
    });
});

describe("OrganisationDeletionSection restoring", () => {
    it("says when the organisation was deleted and until when the server says it can be restored", () => {
        // when
        renderWithProviders(<OrganisationDeletionSection organisation={deletedHarbour} />);

        // then
        const panel = screen.getByRole("region", { name: "Restore organisation" });
        expect(panel).toHaveTextContent(/Harbour was deleted on 20 September 2026/);
        expect(panel).toHaveTextContent(/You can restore it until 20 October 2026/);
        expect(screen.queryByRole("button", { name: "Delete organisation" })).not.toBeInTheDocument();
    });

    it("names no deadline when the answer carries none", () => {
        // given
        const organisation = makeOrganisation({ ...harbour, deleted_at: "2026-09-20T12:00:00Z" });

        // when
        renderWithProviders(<OrganisationDeletionSection organisation={organisation} />);

        // then
        expect(screen.getByRole("region", { name: "Restore organisation" })).not.toHaveTextContent(
            "You can restore it until",
        );
    });

    it("restores the organisation", async () => {
        // given
        const restoration = makeMutationState();
        useRestoreOrganisation.mockReturnValue(restoration);
        const user = userEvent.setup();
        renderWithProviders(<OrganisationDeletionSection organisation={deletedHarbour} />);

        // when
        await user.click(screen.getByRole("button", { name: "Restore organisation" }));

        // then
        expect(restoration.mutate).toHaveBeenCalledExactlyOnceWith({ orgId: "org-harbour" });
    });

    it("says the organisation was restored", () => {
        // given
        useRestoreOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<OrganisationDeletionSection organisation={harbour} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Harbour restored.");
    });

    it("holds the button while the organisation is restored", () => {
        // given
        useRestoreOrganisation.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<OrganisationDeletionSection organisation={deletedHarbour} />);

        // then
        expect(screen.getByRole("button", { name: "Restore organisation" })).toBeDisabled();
    });

    it.each([
        { status: 409, message: "the organisation can no longer be restored" },
        { status: 409, message: "you already own as many organisations as you can" },
        { status: 403, message: "only the owner can restore the organisation" },
    ])("shows the server's refusal as it comes and moves focus to it: $message", refusal => {
        // given
        useRestoreOrganisation.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderWithProviders(<OrganisationDeletionSection organisation={deletedHarbour} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
