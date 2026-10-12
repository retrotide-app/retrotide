import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { OrganisationNameSection } from "./OrganisationNameSection";

const { useUpdateOrganisation } = vi.hoisted(() => ({ useUpdateOrganisation: vi.fn() }));

vi.mock("../../../hooks/mutations/organisations", () => ({ useUpdateOrganisation }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", allow_guests: true });

function renderSection() {
    return renderWithProviders(<OrganisationNameSection organisation={harbour} />);
}

async function openDialog() {
    const user = userEvent.setup();
    renderSection();
    await user.click(screen.getByRole("button", { name: "Rename" }));

    return user;
}

beforeEach(() => {
    vi.clearAllMocks();
    useUpdateOrganisation.mockReturnValue(makeMutationState());
});

describe("OrganisationNameSection", () => {
    it("shows the name with a rename button, and no field until asked", () => {
        // when
        renderSection();

        // then
        expect(screen.getByText("Harbour")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Rename" })).toBeInTheDocument();
        expect(screen.queryByLabelText("Organisation name")).not.toBeInTheDocument();
    });

    it("starts from the organisation's name and saves the new one, keeping the rest as it is", async () => {
        // given
        const update = makeMutationState();
        useUpdateOrganisation.mockReturnValue(update);
        const user = await openDialog();

        // when
        const field = screen.getByLabelText("Organisation name");
        const before = (field as HTMLInputElement).value;
        await user.clear(field);
        await user.type(field, "Harbour Works");
        await user.click(screen.getByRole("button", { name: "Save name" }));

        // then
        expect(before).toBe("Harbour");
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", name: "Harbour Works", slug: "harbour", allow_guests: true },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("says what is missing and ties it to the field, without saving", async () => {
        // given
        const update = makeMutationState();
        useUpdateOrganisation.mockReturnValue(update);
        const user = await openDialog();

        // when
        await user.clear(screen.getByLabelText("Organisation name"));
        await user.click(screen.getByRole("button", { name: "Save name" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter an organisation name.");
        expect(screen.getByLabelText("Organisation name")).toBeInvalid();
        expect(update.mutate).not.toHaveBeenCalled();
    });

    it("says the name was saved", () => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderSection();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Name saved.");
    });

    it("holds the save button while the name is saved", async () => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
    });

    it.each([
        { status: 400, message: "an organisation name is required", code: "org_name_required" },
        { status: 400, message: "an organisation name can be at most 80 characters", code: "org_name_too_long" },
    ])("ties the server's refusal of the name to the field: $code", async refusal => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Organisation name")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
    });

    it.each([
        { status: 404, message: "organisation not found" },
        { status: 403, message: "forbidden" },
        { status: 500, message: "the change was saved, but its cached copy could not be cleared" },
        { status: 409, message: "that address is already taken", code: "slug_taken" },
    ])("shows any other failure as it comes, without blaming the name: $message", async failure => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(failure) }));

        // when
        await openDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(failure.message);
        expect(screen.getByLabelText("Organisation name")).not.toHaveAttribute("aria-invalid");
    });
});
