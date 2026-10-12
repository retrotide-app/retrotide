import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { BoardSection } from "../../../domain/board/organisationBoards";
import { makeOrganisation, makeOrgBoardListItem } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { BoardsSection } from "./BoardsSection";

const harbour = makeOrganisation({ slug: "harbour", name: "Harbour" });

const sprint = makeOrgBoardListItem({ id: "board-sprint", team_id: "team-crew", title: "Sprint 12" });

const launch = makeOrgBoardListItem({ id: "board-launch", team_id: "team-crew", title: "Launch review" });

function makeSection(overrides: Partial<BoardSection> = {}): BoardSection {
    return {
        key: "team-crew",
        teamId: "team-crew",
        title: "Crew",
        hint: "Only people on this team can open these.",
        boards: [sprint, launch],
        canAdd: true,
        archived: false,
        ...overrides,
    };
}

describe("BoardsSection", () => {
    it("heads the section with its title, board count and hint", () => {
        // when
        renderWithProviders(<BoardsSection organisation={harbour} section={makeSection()} display="grid" />);

        // then
        const section = screen.getByRole("region", { name: "Crew" });
        expect(within(section).getByRole("heading", { level: 2, name: "Crew" })).toBeInTheDocument();
        expect(section).toHaveTextContent("2 boards");
        expect(section).toHaveAccessibleDescription("Only people on this team can open these.");
    });

    it("leads with an Add board tile for the team, then each board in order", () => {
        // when
        renderWithProviders(<BoardsSection organisation={harbour} section={makeSection()} display="grid" />);

        // then
        const items = within(screen.getByRole("list", { name: "Crew" })).getAllByRole("listitem");
        expect(items).toHaveLength(3);
        expect(within(items[0]).getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=team-crew",
        );
        expect(within(items[1]).getByRole("link", { name: "Sprint 12" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/board-sprint",
        );
        expect(within(items[2]).getByRole("link", { name: "Launch review" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/board-launch",
        );
    });

    it("opens the wizard with no team from the organisation section", () => {
        // given
        const section = makeSection({ key: "organisation", teamId: null, title: "Organisation boards", boards: [] });

        // when
        renderWithProviders(<BoardsSection organisation={harbour} section={section} display="list" />);

        // then
        expect(screen.getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=none",
        );
        expect(screen.getByRole("region", { name: "Organisation boards" })).toHaveTextContent("0 boards");
    });

    it("offers no tile where the viewer may not add a board, and marks an archived team", () => {
        // when
        renderWithProviders(
            <BoardsSection
                organisation={harbour}
                section={makeSection({ canAdd: false, archived: true, boards: [sprint] })}
                display="grid"
            />,
        );

        // then
        const section = screen.getByRole("region", { name: "Crew" });
        expect(within(section).queryByRole("link", { name: "Add board" })).not.toBeInTheDocument();
        expect(section).toHaveTextContent("1 board");
        expect(within(section).getByText("Archived team")).toBeInTheDocument();
    });

    it("offers neither pins nor selection when it is not given them", () => {
        // when
        renderWithProviders(<BoardsSection organisation={harbour} section={makeSection()} display="grid" />);

        // then
        expect(screen.queryByRole("button", { name: "Pin board" })).not.toBeInTheDocument();
        expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    });

    it("pins a board from its tile", async () => {
        // given
        const onPin = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <BoardsSection organisation={harbour} section={makeSection()} display="grid" onPin={onPin} />,
        );

        // when
        const items = within(screen.getByRole("list", { name: "Crew" })).getAllByRole("listitem");
        await user.click(within(items[1]).getByRole("button", { name: "Pin board" }));

        // then
        expect(onPin).toHaveBeenCalledExactlyOnceWith(sprint);
    });

    it("marks the chosen boards and hands back a board picked from its tile", async () => {
        // given
        const onSelect = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <BoardsSection
                organisation={harbour}
                section={makeSection()}
                display="grid"
                selected={new Set(["board-sprint"])}
                onSelect={onSelect}
            />,
        );

        // when
        await user.click(screen.getByRole("checkbox", { name: "Select Launch review" }));

        // then
        expect(screen.getByRole("checkbox", { name: "Select Sprint 12" })).toBeChecked();
        expect(screen.getByRole("checkbox", { name: "Select Launch review" })).not.toBeChecked();
        expect(onSelect).toHaveBeenCalledExactlyOnceWith(launch);
    });

    it("pins and selects an ended retro too", () => {
        // given
        const ended = makeOrgBoardListItem({ id: "board-old", team_id: "team-crew", title: "Sprint 11", ended: true });

        // when
        renderWithProviders(
            <BoardsSection
                organisation={harbour}
                section={makeSection({ boards: [ended] })}
                display="grid"
                onPin={vi.fn()}
                selected={new Set()}
                onSelect={vi.fn()}
            />,
        );

        // then
        const endedList = screen.getByRole("list", { name: "Ended retros" });
        expect(within(endedList).getByRole("button", { name: "Pin board" })).toBeInTheDocument();
        expect(within(endedList).getByRole("checkbox", { name: "Select Sprint 11" })).toBeInTheDocument();
    });
});
