import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardListItem, makeOrganisation, makeQueryState, makeTeam } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardListResponse } from "../../types/api";
import { TeamBoardsSection } from "./TeamBoardsSection";

const { useTeamBoards } = vi.hoisted(() => ({ useTeamBoards: vi.fn() }));

vi.mock("../../hooks/queries/boards", () => ({ useTeamBoards }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour" });
const crew = makeTeam({ id: "team-crew", name: "Crew" });

function showBoards(boards: BoardListResponse["boards"]) {
    useTeamBoards.mockReturnValue(makeQueryState<BoardListResponse>({ data: { boards } }));
}

function renderSection(createsBoards = true) {
    return renderWithProviders(<TeamBoardsSection organisation={harbour} team={crew} createsBoards={createsBoards} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    showBoards([]);
});

describe("TeamBoardsSection", () => {
    it("starts the grid with an add board tile, then the boards in the order the server sends", () => {
        // given
        showBoards([
            makeBoardListItem({ id: "board-13", title: "Sprint 13" }),
            makeBoardListItem({ id: "board-12", title: "Sprint 12" }),
        ]);

        // when
        renderSection();

        // then
        expect(useTeamBoards).toHaveBeenCalledWith("team-crew");
        const items = within(screen.getByRole("list", { name: "Boards for Crew" })).getAllByRole("listitem");
        expect(items).toHaveLength(3);
        expect(within(items[0]).getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=team-crew",
        );
        expect(within(items[1]).getByRole("link", { name: "Sprint 13" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/board-13",
        );
        expect(within(items[2]).getByRole("link", { name: "Sprint 12" })).toBeInTheDocument();
    });

    it("counts the boards next to the heading", () => {
        // given
        showBoards([makeBoardListItem({ id: "board-12" })]);

        // when
        renderSection();

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Boards" })).toBeInTheDocument();
        expect(screen.getByText("1 board")).toBeInTheDocument();
    });

    it("shows only the add board tile when the team has no boards yet", () => {
        // when
        renderSection();

        // then
        const items = within(screen.getByRole("list", { name: "Boards for Crew" })).getAllByRole("listitem");
        expect(items).toHaveLength(1);
        expect(screen.getByText("0 boards")).toBeInTheDocument();
    });

    it("leaves the tile out for a viewer who may not create boards", () => {
        // given
        showBoards([makeBoardListItem({ id: "board-12", title: "Sprint 12" })]);

        // when
        renderSection(false);

        // then
        expect(screen.queryByRole("link", { name: "Add board" })).not.toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Sprint 12" })).toBeInTheDocument();
    });

    it("says the team has no boards when the viewer can neither see nor add any", () => {
        // when
        renderSection(false);

        // then
        expect(screen.getByText("This team has no boards yet.")).toBeInTheDocument();
        expect(screen.queryByRole("list", { name: "Boards for Crew" })).not.toBeInTheDocument();
    });

    it("says it is loading while the boards are on their way", () => {
        // given
        useTeamBoards.mockReturnValue(makeQueryState<BoardListResponse>({ loading: true }));

        // when
        renderSection();

        // then
        expect(screen.getByText("Loading boards...")).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Add board" })).not.toBeInTheDocument();
    });

    it("shows the server's reason when the boards could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<BoardListResponse>({
            error: makeApiError({ status: 500, message: "boards unavailable" }),
        });
        useTeamBoards.mockReturnValue(failed);
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("boards unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
