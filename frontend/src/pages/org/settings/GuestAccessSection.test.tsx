import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { GuestAccessSection } from "./GuestAccessSection";

const { useUpdateOrganisation } = vi.hoisted(() => ({ useUpdateOrganisation: vi.fn() }));

vi.mock("../../../hooks/mutations/organisations", () => ({ useUpdateOrganisation }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", allow_guests: true });

function renderSection(allowGuests = true) {
    return renderWithProviders(<GuestAccessSection organisation={{ ...harbour, allow_guests: allowGuests }} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    useUpdateOrganisation.mockReturnValue(makeMutationState());
});

describe("GuestAccessSection", () => {
    it.each([true, false])("shows whether guests are allowed: %s", allowGuests => {
        // when
        renderSection(allowGuests);

        // then
        expect(screen.getByRole("switch", { name: "Allow guests" })).toHaveAttribute(
            "aria-checked",
            String(allowGuests),
        );
    });

    it("turns guests off, keeping the rest as it is", async () => {
        // given
        const update = makeMutationState();
        useUpdateOrganisation.mockReturnValue(update);
        const user = userEvent.setup();
        renderSection(true);

        // when
        await user.click(screen.getByRole("switch", { name: "Allow guests" }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith({
            orgId: "org-harbour",
            name: "Harbour",
            slug: "harbour",
            allow_guests: false,
        });
    });

    it("turns guests on", async () => {
        // given
        const update = makeMutationState();
        useUpdateOrganisation.mockReturnValue(update);
        const user = userEvent.setup();
        renderSection(false);

        // when
        await user.click(screen.getByRole("switch", { name: "Allow guests" }));

        // then
        expect(update.mutate).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ allow_guests: true }));
    });

    it("holds the switch while the change is saved", () => {
        // given
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderSection();

        // then
        expect(screen.getByRole("switch", { name: "Allow guests" })).toBeDisabled();
    });

    it.each([
        { allowGuests: true, message: "Guests are now allowed." },
        { allowGuests: false, message: "Guests are no longer allowed." },
    ])("says what changed: $message", ({ allowGuests, message }) => {
        // given
        useUpdateOrganisation.mockReturnValue(
            makeMutationState({
                isSuccess: true,
                variables: { orgId: "org-harbour", name: "Harbour", slug: "harbour", allow_guests: allowGuests },
            }),
        );

        // when
        renderSection(allowGuests);

        // then
        expect(screen.getByRole("status")).toHaveTextContent(message);
    });

    it("shows the server's refusal as it comes and moves focus to it", () => {
        // given
        const error = makeApiError({ status: 403, message: "forbidden" });
        useUpdateOrganisation.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderSection();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("forbidden");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
