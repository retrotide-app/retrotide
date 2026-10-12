import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeBoardParticipant,
    makeBoardSnapshot,
    makeBoardView,
    makeMutationState,
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { PeoplePanel } from "./PeoplePanel";

const mutations = vi.hoisted(() => ({
    useSetParticipantRole: vi.fn(),
    useRemoveParticipant: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => mutations);

const people = [
    makeBoardParticipant(),
    makeBoardParticipant({ participant_id: "p2", display_name: "Ali", role: "participant" }),
];

beforeEach(() => {
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
});

describe("PeoplePanel", () => {
    it("lists only the people on the board right now, with the count", () => {
        // given
        const snapshot = makeBoardSnapshot({
            participants: [...people, makeBoardParticipant({ participant_id: "p3", display_name: "Gone" })],
        });

        // when
        renderWithProviders(<PeoplePanel snapshot={snapshot} online={new Set(["p2"])} />);

        // then the viewer always counts as here, and someone who left is not listed
        const panel = screen.getByRole("complementary", { name: /Here now/ });
        expect(within(panel).getByRole("heading", { name: "Here now 2" })).toBeInTheDocument();
        expect(within(panel).getByRole("list", { name: "People on this board" })).toHaveTextContent("Ali");
        expect(within(panel).queryByText("Gone")).not.toBeInTheDocument();
    });

    it("shows just the viewer before the first presence update arrives", () => {
        // when
        renderWithProviders(<PeoplePanel snapshot={makeBoardSnapshot({ participants: people })} online={null} />);

        // then
        expect(screen.getByRole("heading", { name: "Here now 1" })).toBeInTheDocument();
        expect(screen.queryByText("Ali")).not.toBeInTheDocument();
    });

    it("shows a participant who is on the board without offering them any way to manage it", () => {
        // given
        const snapshot = makeBoardSnapshot({
            participants: people,
            board: makeBoardView(),
            viewer: makeParticipantViewer(),
        });

        // when
        renderWithProviders(<PeoplePanel snapshot={snapshot} online={new Set(["p2"])} />);

        // then
        expect(screen.getByText("Ali")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: /Actions for/ })).not.toBeInTheDocument();
    });
});
