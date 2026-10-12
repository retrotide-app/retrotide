import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import {
    makeApiError,
    makeBoardListItem,
    makeBoardSnapshot,
    makeBoardTemplate,
    makeOrgBoardListItem,
} from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useBoard, useBoardTemplate, useBoardTemplates, useOrganisationBoards, useTeamBoards } from "./boards";

const endpoints = vi.hoisted(() => ({
    getBoard: vi.fn(),
    listOrganisationBoards: vi.fn(),
    listTeamBoards: vi.fn(),
}));

const templateEndpoints = vi.hoisted(() => ({
    listBoardTemplates: vi.fn(),
    getBoardTemplate: vi.fn(),
}));

vi.mock("../../api/endpoints/boards", () => endpoints);
vi.mock("../../api/endpoints/boardTemplates", () => templateEndpoints);

function setup<T>(hook: () => T) {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(hook, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getBoard.mockReset();
    endpoints.listTeamBoards.mockReset();
    endpoints.listOrganisationBoards.mockReset();
    templateEndpoints.listBoardTemplates.mockReset();
    templateEndpoints.getBoardTemplate.mockReset();
    endpoints.listTeamBoards.mockResolvedValue({ boards: [] });
    endpoints.getBoard.mockResolvedValue(makeBoardSnapshot());
    templateEndpoints.listBoardTemplates.mockResolvedValue({ templates: [] });
});

describe("useTeamBoards", () => {
    it("reads the team's boards under the team's board list key", async () => {
        // given
        const listing = { boards: [makeBoardListItem(), makeBoardListItem({ id: "b2", created_at: undefined })] };
        endpoints.listTeamBoards.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useTeamBoards("t1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listTeamBoards).toHaveBeenCalledWith("t1");
        expect(queryClient.getQueryData(queryKeys.boards.team("t1"))).toEqual(listing);
    });

    it("hands back the error when the team's boards could not be read", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "team not found" });
        endpoints.listTeamBoards.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useTeamBoards("t1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });

    it("asks nothing without a team id", () => {
        // when
        const { result } = setup(() => useTeamBoards(""));

        // then
        expect(endpoints.listTeamBoards).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});

describe("useOrganisationBoards", () => {
    it("reads the organisation's boards under the organisation's board list key", async () => {
        // given
        const listing = { boards: [makeOrgBoardListItem(), makeOrgBoardListItem({ id: "b2", team_name: "Deck" })] };
        endpoints.listOrganisationBoards.mockResolvedValue(listing);

        // when
        const { result, queryClient } = setup(() => useOrganisationBoards("o1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(listing));
        expect(endpoints.listOrganisationBoards).toHaveBeenCalledWith("o1");
        expect(queryClient.getQueryData(queryKeys.boards.organisation("o1"))).toEqual(listing);
    });

    it("hands back the error when the organisation's boards could not be read", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "organisation not found" });
        endpoints.listOrganisationBoards.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useOrganisationBoards("o1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });

    it("asks nothing without an organisation id", () => {
        // when
        const { result } = setup(() => useOrganisationBoards(""));

        // then
        expect(endpoints.listOrganisationBoards).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });
});

describe("useBoard", () => {
    it("reads the whole board under its live state key when nothing is held", async () => {
        // given
        const snapshot = makeBoardSnapshot({ board_id: "b1", version: 3 });
        endpoints.getBoard.mockResolvedValue(snapshot);

        // when
        const { result, queryClient } = setup(() => useBoard("b1"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(snapshot));
        expect(endpoints.getBoard).toHaveBeenCalledWith("b1", undefined, undefined);
        expect(queryClient.getQueryData(queryKeys.boardLive.state("b1"))).toEqual(snapshot);
    });

    it("sends the version it holds on a refetch and keeps the board when the server says it is unchanged", async () => {
        // given
        const snapshot = makeBoardSnapshot({ board_id: "b1", version: 3 });
        endpoints.getBoard.mockResolvedValueOnce(snapshot).mockResolvedValueOnce(undefined);
        const { result, queryClient } = setup(() => useBoard("b1"));
        await waitFor(() => expect(result.current.data).toEqual(snapshot));

        // when
        await act(async () => {
            await result.current.refetch();
        });

        // then
        expect(endpoints.getBoard).toHaveBeenCalledTimes(2);
        expect(endpoints.getBoard).toHaveBeenLastCalledWith("b1", 3, undefined);
        expect(queryClient.getQueryData(queryKeys.boardLive.state("b1"))).toBe(snapshot);
        expect(queryClient.getQueryState(queryKeys.boardLive.state("b1"))?.error).toBeNull();
    });

    it("takes a newer snapshot on a refetch", async () => {
        // given
        const first = makeBoardSnapshot({ board_id: "b1", version: 3 });
        const newer = makeBoardSnapshot({ board_id: "b1", version: 4, cards: [] });
        endpoints.getBoard.mockResolvedValueOnce(first).mockResolvedValueOnce(newer);
        const { result } = setup(() => useBoard("b1"));
        await waitFor(() => expect(result.current.data).toEqual(first));

        // when
        await act(async () => {
            await result.current.refetch();
        });

        // then
        await waitFor(() => expect(result.current.data).toEqual(newer));
    });

    it("keeps the board it holds when a slower answer arrives with an older version", async () => {
        // given
        const held = makeBoardSnapshot({ board_id: "b1", version: 6 });
        const older = makeBoardSnapshot({ board_id: "b1", version: 5, cards: [] });
        endpoints.getBoard.mockResolvedValueOnce(held).mockResolvedValueOnce(older);
        const { result, queryClient } = setup(() => useBoard("b1"));
        await waitFor(() => expect(result.current.data).toEqual(held));

        // when
        await act(async () => {
            await result.current.refetch();
        });

        // then
        expect(endpoints.getBoard).toHaveBeenCalledTimes(2);
        expect(queryClient.getQueryData(queryKeys.boardLive.state("b1"))).toBe(held);
    });

    it("says so when the server answers unchanged before any board is held", async () => {
        // given
        endpoints.getBoard.mockResolvedValue(undefined);

        // when
        const { result } = setup(() => useBoard("b1"));

        // then
        await waitFor(() => expect(result.current.error?.message).toContain("answered unchanged"));
    });

    it("hands back the error when the board could not be read", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "board not found" });
        endpoints.getBoard.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useBoard("b1"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });

    it("asks nothing without a board id", () => {
        // when
        setup(() => useBoard(""));

        // then
        expect(endpoints.getBoard).not.toHaveBeenCalled();
    });
});

describe("useBoardTemplates", () => {
    it("reads the template catalogue under its key", async () => {
        // given
        const catalogue = {
            templates: [makeBoardTemplate({ key: "mad-sad-glad", name: "Mad Sad Glad" }), makeBoardTemplate()],
        };
        templateEndpoints.listBoardTemplates.mockResolvedValue(catalogue);

        // when
        const { result, queryClient } = setup(() => useBoardTemplates());

        // then
        await waitFor(() => expect(result.current.data).toEqual(catalogue));
        expect(queryClient.getQueryData(queryKeys.boardTemplates())).toEqual(catalogue);
    });

    it("hands back the error when the catalogue could not be read", async () => {
        // given
        const failure = makeApiError({ status: 401, message: "authentication required" });
        templateEndpoints.listBoardTemplates.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useBoardTemplates());

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});

describe("useBoardTemplate", () => {
    it("reads one template under its own key", async () => {
        // given
        const sailboat = makeBoardTemplate({ key: "sailboat", name: "Sailboat" });
        templateEndpoints.getBoardTemplate.mockResolvedValue(sailboat);

        // when
        const { result, queryClient } = setup(() => useBoardTemplate("sailboat"));

        // then
        await waitFor(() => expect(result.current.data).toEqual(sailboat));
        expect(templateEndpoints.getBoardTemplate).toHaveBeenCalledWith("sailboat");
        expect(queryClient.getQueryData(queryKeys.boardTemplate("sailboat"))).toEqual(sailboat);
    });

    it("hands back the error for an unknown template", async () => {
        // given
        const failure = makeApiError({ status: 404, message: "template not found" });
        templateEndpoints.getBoardTemplate.mockRejectedValue(failure);

        // when
        const { result } = setup(() => useBoardTemplate("no-such-template"));

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });

    it("asks for nothing until it has a key", () => {
        // when
        setup(() => useBoardTemplate(""));

        // then
        expect(templateEndpoints.getBoardTemplate).not.toHaveBeenCalled();
    });
});
