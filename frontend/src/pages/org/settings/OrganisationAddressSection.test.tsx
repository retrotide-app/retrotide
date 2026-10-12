import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { OrganisationAddressSection } from "./OrganisationAddressSection";

const { useUpdateOrganisation } = vi.hoisted(() => ({ useUpdateOrganisation: vi.fn() }));

vi.mock("../../../hooks/mutations/organisations", () => ({ useUpdateOrganisation }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", allow_guests: false });

function renderSection() {
    return renderWithProviders(<OrganisationAddressSection organisation={harbour} />);
}

async function openDialog() {
    const user = userEvent.setup();
    renderSection();
    await user.click(screen.getByRole("button", { name: "Change address" }));

    return user;
}

beforeEach(() => {
    vi.clearAllMocks();
    useUpdateOrganisation.mockReturnValue(makeMutationState());
});

describe("OrganisationAddressSection", () => {
    it("shows the address with a change button, and no field until asked", () => {
        // when
        renderSection();

        // then
        expect(screen.getByText("harbour")).toBeInTheDocument();
        expect(screen.queryByLabelText("Organisation address")).not.toBeInTheDocument();
    });

    it("starts from the organisation's address and saves the new one, keeping the rest as it is", async () => {
        // given
        const update = makeMutationState();
        useUpdateOrganisation.mockReturnValue(update);
        const user = await openDialog();

        // when
        const field = screen.getByLabelText("Organisation address");
        const before = (field as HTMLInputElement).value;
        await user.clear(field);
        await user.type(field, "harbour-works");
        await user.click(screen.getByRole("button", { name: "Save address" }));

        // then
        expect(before).toBe("harbour");
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", name: "Harbour", slug: "harbour-works", allow_guests: false },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("says what an address may hold and that old links stop working", async () => {
        // when
        await openDialog();

        // then
        expect(screen.getByLabelText("Organisation address")).toHaveAccessibleDescription(
            "3 to 40 lower-case letters, digits or hyphens. Links that use the old address stop working.",
        );
    });

    it("says what is missing and ties it to the field, without saving", async () => {
        // given
        const update = makeMutationState();
        useUpdateOrganisation.mockReturnValue(update);
        const user = await openDialog();

        // when
        await user.clear(screen.getByLabelText("Organisation address"));
        await user.click(screen.getByRole("button", { name: "Save address" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter an address.");
        expect(screen.getByLabelText("Organisation address")).toBeInvalid();
        expect(update.mutate).not.toHaveBeenCalled();
    });

    it("says the address was saved", () => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderSection();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Address saved.");
    });

    it("holds the save button while the address is saved", async () => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("button", { name: "Save address" })).toBeDisabled();
    });

    it.each([
        {
            status: 400,
            message:
                "the address must be 3 to 40 lower-case letters, digits or hyphens, starting and ending with a letter or digit",
            code: "slug_invalid",
        },
        { status: 400, message: "that address is reserved", code: "slug_reserved" },
        { status: 409, message: "that address is already taken", code: "slug_taken" },
    ])("ties the server's refusal of the address to the field: $code", async refusal => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Organisation address")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
    });

    it.each([
        { status: 404, message: "organisation not found" },
        { status: 400, message: "an organisation name can be at most 80 characters", code: "org_name_too_long" },
    ])("shows any other failure as it comes, without blaming the address: $message", async refusal => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Organisation address")).not.toHaveAttribute("aria-invalid");
    });
});
