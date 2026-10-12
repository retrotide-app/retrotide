import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeOrganisation, makeOrgBoardListItem } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BulkBoardResponse } from "../../../types/api";
import { BulkBoardBar } from "./BulkBoardBar";

const { useBulkBoards, useBulkExportUrl } = vi.hoisted(() => ({
    useBulkBoards: vi.fn(),
    useBulkExportUrl: vi.fn(),
}));

vi.mock("../../../hooks/mutations/boardManagement", () => ({ useBulkBoards }));
vi.mock("../../../hooks/queries/boardManagement", () => ({ useBulkExportUrl }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour" });

const sprint = makeOrgBoardListItem({ id: "b1", title: "Sprint 12" });

const launch = makeOrgBoardListItem({ id: "b2", title: "Launch review" });

const EXPORT_URL = "/api/v1/orgs/org-harbour/boards/export?board_id=b1&board_id=b2";

function renderBar(selected: ReadonlySet<string> = new Set(["b1", "b2"])) {
    const onSelectionChange = vi.fn();

    renderWithProviders(
        <BulkBoardBar
            organisation={harbour}
            boards={[sprint, launch]}
            selected={selected}
            onSelectionChange={onSelectionChange}
        />,
    );

    return { onSelectionChange, bar: screen.getByRole("region", { name: "Selected boards" }) };
}

function answeringWith(response: BulkBoardResponse) {
    const bulk = makeMutationState();
    bulk.mutate.mockImplementation(
        (_variables: unknown, options: { onSuccess: (answer: BulkBoardResponse) => void }) => {
            options.onSuccess(response);
        },
    );
    useBulkBoards.mockReturnValue(bulk);

    return bulk;
}

beforeEach(() => {
    vi.clearAllMocks();
    useBulkBoards.mockReturnValue(makeMutationState());
    useBulkExportUrl.mockReturnValue(EXPORT_URL);
});

describe("BulkBoardBar", () => {
    it("counts the chosen boards and links their export", () => {
        // when
        const { bar } = renderBar();

        // then
        expect(bar).toHaveTextContent("2 boards selected");
        expect(within(bar).getByRole("link", { name: "Export" })).toHaveAttribute("href", EXPORT_URL);
        expect(useBulkExportUrl).toHaveBeenCalledWith("org-harbour", ["b1", "b2"]);
        expect(useBulkBoards).toHaveBeenCalledWith("org-harbour");
    });

    it("offers nothing to act on while no board is chosen", () => {
        // when
        const { bar } = renderBar(new Set());

        // then
        expect(bar).toHaveTextContent("0 boards selected");
        expect(within(bar).getByRole("button", { name: "Archive" })).toBeDisabled();
        expect(within(bar).getByRole("button", { name: "Unarchive" })).toBeDisabled();
        expect(within(bar).getByRole("button", { name: "Export" })).toBeDisabled();
        expect(within(bar).getByRole("button", { name: "Delete" })).toBeDisabled();
    });

    it.each([
        { label: "Archive", action: "archive" },
        { label: "Unarchive", action: "unarchive" },
    ])("sends $label for every chosen board and clears the choice once all succeed", async ({ label, action }) => {
        // given
        const bulk = answeringWith({
            results: [
                { board_id: "b1", ok: true },
                { board_id: "b2", ok: true },
            ],
        });
        const user = userEvent.setup();
        const { bar, onSelectionChange } = renderBar();

        // when
        await user.click(within(bar).getByRole("button", { name: label }));

        // then
        expect(bulk.mutate).toHaveBeenCalledWith({ action, boardIds: ["b1", "b2"] }, expect.anything());
        expect(onSelectionChange).toHaveBeenCalledExactlyOnceWith(new Set());
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("asks before deleting, saying the boards can be restored for 30 days", async () => {
        // given
        const bulk = answeringWith({
            results: [
                { board_id: "b1", ok: true },
                { board_id: "b2", ok: true },
            ],
        });
        const user = userEvent.setup();
        const { bar } = renderBar();

        // when
        await user.click(within(bar).getByRole("button", { name: "Delete" }));
        const dialog = screen.getByRole("dialog", { name: "Delete boards" });
        await user.click(within(dialog).getByRole("button", { name: "Delete boards" }));

        // then
        expect(dialog).toHaveTextContent("You can restore them from Recently deleted for 30 days.");
        expect(bulk.mutate).toHaveBeenCalledWith({ action: "delete", boardIds: ["b1", "b2"] }, expect.anything());
    });

    it("deletes nothing when the question is cancelled", async () => {
        // given
        const bulk = makeMutationState();
        useBulkBoards.mockReturnValue(bulk);
        const user = userEvent.setup();
        const { bar } = renderBar();

        // when
        await user.click(within(bar).getByRole("button", { name: "Delete" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete boards" })).getByRole("button", { name: "Cancel" }),
        );

        // then
        expect(bulk.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("dialog", { name: "Delete boards" })).not.toBeInTheDocument();
    });

    it("names each board it could not change and keeps only those chosen", async () => {
        // given
        answeringWith({
            results: [
                { board_id: "b1", ok: true },
                { board_id: "b2", ok: false, error: "your role on this board does not allow that" },
            ],
        });
        const user = userEvent.setup();
        const { bar, onSelectionChange } = renderBar();

        // when
        await user.click(within(bar).getByRole("button", { name: "Archive" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "These boards could not be changed, so they stay selected:",
        );
        expect(screen.getByRole("alert")).toHaveTextContent(
            "Launch review: your role on this board does not allow that",
        );
        expect(onSelectionChange).toHaveBeenCalledExactlyOnceWith(new Set(["b2"]));
    });

    it("shows why the whole request was refused", () => {
        // given
        useBulkBoards.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 400,
                    message: "a bulk action takes at most 100 boards",
                    code: "too_many_boards",
                }),
            }),
        );

        // when
        renderBar();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("a bulk action takes at most 100 boards");
    });

    it("clears the choice on request", async () => {
        // given
        const user = userEvent.setup();
        const { bar, onSelectionChange } = renderBar();

        // when
        await user.click(within(bar).getByRole("button", { name: "Clear selection" }));

        // then
        expect(onSelectionChange).toHaveBeenCalledExactlyOnceWith(new Set());
    });
});
