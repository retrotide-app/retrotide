import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeOrganisation, makeQueryState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { DeletedBoardItem, DeletedBoardListResponse } from "../../../types/api";
import { DeletedBoardsSection } from "./DeletedBoardsSection";

const { useDeletedBoards, useRestoreBoard } = vi.hoisted(() => ({
    useDeletedBoards: vi.fn(),
    useRestoreBoard: vi.fn(),
}));

vi.mock("../../../hooks/queries/boardManagement", () => ({ useDeletedBoards }));
vi.mock("../../../hooks/mutations/boardManagement", () => ({ useRestoreBoard }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour" });

const teamBoard: DeletedBoardItem = {
    id: "b1",
    team_id: "t1",
    team_name: "Crew",
    title: "Sprint 11",
    deleted_at: "2026-10-09T09:00:00.000Z",
    restore_until: "2026-11-08T09:00:00.000Z",
};

const organisationBoard: DeletedBoardItem = {
    id: "b2",
    title: "Town hall",
    deleted_at: "2026-10-10T09:00:00.000Z",
    restore_until: "2026-11-09T09:00:00.000Z",
};

function showDeleted(boards: DeletedBoardItem[]) {
    useDeletedBoards.mockReturnValue(makeQueryState<DeletedBoardListResponse>({ data: { boards } }));
}

beforeEach(() => {
    vi.clearAllMocks();
    showDeleted([teamBoard, organisationBoard]);
    useRestoreBoard.mockReturnValue(makeMutationState());
});

describe("DeletedBoardsSection", () => {
    it("lists each deleted board with its team, when it was deleted and until when it can come back", () => {
        // when
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // then
        const section = screen.getByRole("region", { name: "Recently deleted" });
        expect(section).toHaveTextContent("2 boards");
        const rows = within(screen.getByRole("table")).getAllByRole("row");
        expect(rows).toHaveLength(3);
        expect(rows[1]).toHaveTextContent("Sprint 11");
        expect(rows[1]).toHaveTextContent("Crew");
        expect(rows[1].querySelectorAll("time")[0]).toHaveAttribute("dateTime", "2026-10-09T09:00:00.000Z");
        expect(rows[1].querySelectorAll("time")[1]).toHaveAttribute("dateTime", "2026-11-08T09:00:00.000Z");
        expect(rows[2]).toHaveTextContent("Organisation boards");
        expect(useDeletedBoards).toHaveBeenCalledWith("org-harbour", true);
        expect(useRestoreBoard).toHaveBeenCalledWith("org-harbour");
    });

    it("restores a board and says so", async () => {
        // given
        const restore = makeMutationState();
        restore.mutate.mockImplementation((_boardId: string, options: { onSuccess: () => void }) => {
            options.onSuccess();
        });
        useRestoreBoard.mockReturnValue(restore);
        const user = userEvent.setup();
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // when
        await user.click(screen.getByRole("button", { name: "Restore Sprint 11" }));

        // then
        expect(restore.mutate).toHaveBeenCalledWith("b1", expect.anything());
        expect(screen.getByRole("status")).toHaveTextContent("Restored Sprint 11.");
    });

    it("shows why a restore was refused", () => {
        // given
        useRestoreBoard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 403,
                    message: "you cannot restore that board",
                    code: "restore_not_allowed",
                }),
            }),
        );

        // when
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("you cannot restore that board");
    });

    it("holds every Restore while one is on its way", () => {
        // given
        useRestoreBoard.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // then
        expect(screen.getByRole("button", { name: "Restore Sprint 11" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Restore Town hall" })).toBeDisabled();
    });

    it("says when there is nothing to restore", () => {
        // given
        showDeleted([]);

        // when
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // then
        expect(screen.getByText("No deleted boards to restore.")).toBeInTheDocument();
        expect(screen.queryByRole("table")).not.toBeInTheDocument();
    });

    it("says the list is loading until it arrives", () => {
        // given
        useDeletedBoards.mockReturnValue(makeQueryState<DeletedBoardListResponse>({ loading: true }));

        // when
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // then
        expect(screen.getByText("Loading deleted boards...")).toBeInTheDocument();
    });

    it("shows why the list could not be read and asks again on request", async () => {
        // given
        const refetch = vi.fn().mockResolvedValue(undefined);
        useDeletedBoards.mockReturnValue(
            makeQueryState<DeletedBoardListResponse>({
                error: makeApiError({ status: 500, message: "failed to list the deleted boards" }),
                refetch,
            }),
        );
        const user = userEvent.setup();
        renderWithProviders(<DeletedBoardsSection organisation={harbour} />);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the deleted boards");
        expect(refetch).toHaveBeenCalledOnce();
    });
});
