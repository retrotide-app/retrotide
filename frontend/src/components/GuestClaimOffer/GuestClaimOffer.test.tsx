import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeQueryState,
    makeSessionUser,
    type MutationStateFixture,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { GuestClaimOfferResponse } from "../../types/api";
import { GuestClaimOffer } from "./GuestClaimOffer";

const queries = vi.hoisted(() => ({ useGuestClaimOffer: vi.fn() }));

const mutations = vi.hoisted(() => ({
    useClaimGuestBoards: vi.fn(),
    useDeclineGuestClaim: vi.fn(),
}));

vi.mock("../../hooks/queries/guestClaim", () => queries);
vi.mock("../../hooks/mutations/guestClaim", () => mutations);

const sam = makeSessionUser({ id: "u1" });

const offer: GuestClaimOfferResponse = {
    boards: [
        { board_id: "b1", title: "Sprint 12", display_name: "Robin" },
        { board_id: "b2", title: "Release retro", display_name: "Anonymous" },
    ],
};

let claim: MutationStateFixture;
let decline: MutationStateFixture;

function renderOffer(signedIn = true) {
    return renderWithProviders(<GuestClaimOffer />, { user: signedIn ? sam : null });
}

beforeEach(() => {
    vi.clearAllMocks();
    claim = makeMutationState();
    decline = makeMutationState();
    mutations.useClaimGuestBoards.mockReturnValue(claim);
    mutations.useDeclineGuestClaim.mockReturnValue(decline);
    queries.useGuestClaimOffer.mockReturnValue(makeQueryState({ data: offer }));
});

describe("GuestClaimOffer", () => {
    it("asks for the offer of the signed in user", () => {
        // when
        renderOffer();

        // then
        expect(queries.useGuestClaimOffer).toHaveBeenCalledWith("u1");
    });

    it("asks nothing and shows nothing while nobody is signed in", () => {
        // when
        renderOffer(false);

        // then
        expect(queries.useGuestClaimOffer).toHaveBeenCalledWith(null);
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("lists each board this browser joined as a guest, with the name used on it", () => {
        // when
        renderOffer();

        // then
        expect(screen.getByRole("dialog", { name: "Bring your guest cards with you?" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Sprint 12" })).toHaveAttribute("aria-checked", "true");
        expect(screen.getByText("You joined as Robin")).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Release retro" })).toBeInTheDocument();
    });

    it("shows nothing when there is nothing to bring over", () => {
        // given
        queries.useGuestClaimOffer.mockReturnValue(makeQueryState({ data: { boards: [] } }));

        // when
        renderOffer();

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("claims every board left switched on", async () => {
        // given
        const user = userEvent.setup();
        renderOffer();

        // when
        await user.click(screen.getByRole("switch", { name: "Release retro" }));
        await user.click(screen.getByRole("button", { name: "Bring them over" }));

        // then
        expect(claim.mutate).toHaveBeenCalledWith(["b1"]);
        expect(decline.mutate).not.toHaveBeenCalled();
    });

    it("declines when every board was switched off", async () => {
        // given
        const user = userEvent.setup();
        renderOffer();

        // when
        await user.click(screen.getByRole("switch", { name: "Sprint 12" }));
        await user.click(screen.getByRole("switch", { name: "Release retro" }));
        await user.click(screen.getByRole("button", { name: "Bring them over" }));

        // then
        expect(claim.mutate).not.toHaveBeenCalled();
        expect(decline.mutate).toHaveBeenCalled();
    });

    it("declines with No thanks", async () => {
        // given
        const user = userEvent.setup();
        renderOffer();

        // when
        await user.click(screen.getByRole("button", { name: "No thanks" }));

        // then
        expect(decline.mutate).toHaveBeenCalled();
    });

    it("says why the claim failed", () => {
        // given
        mutations.useClaimGuestBoards.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 503, message: "the board is busy" }) }),
        );

        // when
        renderOffer();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the board is busy");
    });

    it("confirms the cards moved once the claim went through", () => {
        // given
        mutations.useClaimGuestBoards.mockReturnValue(
            makeMutationState({ isSuccess: true, data: { claimed: ["b1"] } }),
        );

        // when
        renderOffer();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Your guest cards are now in your account.");
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("says the offer could not be checked rather than hiding the failure", () => {
        // given
        queries.useGuestClaimOffer.mockReturnValue(
            makeQueryState({ error: makeApiError({ status: 503, message: "service unavailable" }) }),
        );

        // when
        renderOffer();

        // then
        expect(screen.getByRole("status")).toHaveTextContent(
            "Checking for boards this browser joined as a guest failed: service unavailable",
        );
    });
});
