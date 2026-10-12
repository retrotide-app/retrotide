import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { rejoinSlugFrom } from "../../domain/board/shareLink";
import type { QueryState } from "../../hooks/queries/queryState";
import {
    BOARD_ID,
    makeApiError,
    makeMutationState,
    makeQueryState,
    makeSessionUser,
    type MutationStateFixture,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { JoinBoardResponse, JoinPreviewResponse } from "../../types/api";
import { JoinBoardPage } from "./JoinBoardPage";

const queries = vi.hoisted(() => ({
    useJoinPreview: vi.fn(),
    useBoardSharing: vi.fn(),
}));

const mutations = vi.hoisted(() => ({
    useJoinBoardByLink: vi.fn(),
    useSetLinkAccess: vi.fn(),
    useSetBoardPassword: vi.fn(),
    useRotateShareLink: vi.fn(),
}));

vi.mock("../../hooks/queries/shareLinks", () => queries);
vi.mock("../../hooks/mutations/shareLinks", () => mutations);

const open: JoinPreviewResponse = { requires_password: false, requires_account: false, can_join: true };

interface JoinCallbacks {
    onSuccess?: (joined: JoinBoardResponse) => void;
}

function BoardProbe() {
    const location = useLocation();

    return <h1>{`board ${location.pathname} via ${rejoinSlugFrom(location.state) ?? "nothing"}`}</h1>;
}

function showPreview(preview: Partial<QueryState<JoinPreviewResponse>>) {
    const state = makeQueryState<JoinPreviewResponse>(preview);
    queries.useJoinPreview.mockReturnValue(state);

    return state;
}

function joinSucceeds(join: MutationStateFixture): void {
    join.mutate.mockImplementation((_request: unknown, callbacks?: JoinCallbacks) => {
        callbacks?.onSuccess?.({ board_id: BOARD_ID });
    });
}

function renderPage(signedIn = false) {
    return renderWithProviders(
        <Routes>
            <Route path="/b/:slug" element={<JoinBoardPage />} />
            <Route path="/boards/:boardId" element={<BoardProbe />} />
        </Routes>,
        { route: "/b/k7Q2", user: signedIn ? makeSessionUser({ display_name: "Sam" }) : null },
    );
}

let join: MutationStateFixture;

beforeEach(() => {
    vi.clearAllMocks();
    join = makeMutationState();
    mutations.useJoinBoardByLink.mockReturnValue(join);
    showPreview({ data: open });
});

describe("JoinBoardPage", () => {
    it("reads the preview for the slug in the address", () => {
        // when
        renderPage();

        // then
        expect(queries.useJoinPreview).toHaveBeenCalledWith("k7Q2");
        expect(mutations.useJoinBoardByLink).toHaveBeenCalledWith("k7Q2");
        expect(screen.getByRole("heading", { name: "Join a retrospective" })).toBeInTheDocument();
    });

    it("says it is loading until the preview is in", () => {
        // given
        showPreview({ loading: true });

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Join board" })).not.toBeInTheDocument();
    });

    it("joins a guest under the name they typed and opens the board, remembering the link for a rejoin", async () => {
        // given
        joinSucceeds(join);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.type(screen.getByLabelText("Your name"), "  Robin ");
        await user.click(screen.getByRole("button", { name: "Join board" }));

        // then
        expect(join.mutate).toHaveBeenCalledWith({ display_name: "Robin" }, expect.anything());
        expect(screen.getByRole("heading", { name: `board /boards/${BOARD_ID} via k7Q2` })).toBeInTheDocument();
    });

    it("joins a guest as Anonymous without a name", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("switch", { name: "Join as Anonymous" }));
        await user.click(screen.getByRole("button", { name: "Join board" }));

        // then
        expect(screen.queryByLabelText("Your name")).not.toBeInTheDocument();
        expect(join.mutate).toHaveBeenCalledWith({}, expect.anything());
    });

    it("asks a guest for a name, or Anonymous, before sending anything", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Join board" }));

        // then
        expect(screen.getByText("Enter your name, or choose to join as Anonymous.")).toBeInTheDocument();
        expect(join.mutate).not.toHaveBeenCalled();
    });

    it("joins someone signed in under their account, asking no name", async () => {
        // given
        const user = userEvent.setup();
        renderPage(true);

        // when
        await user.click(screen.getByRole("button", { name: "Join board" }));

        // then
        expect(screen.getByText("Sam")).toBeInTheDocument();
        expect(screen.queryByLabelText("Your name")).not.toBeInTheDocument();
        expect(screen.queryByRole("switch", { name: "Join as Anonymous" })).not.toBeInTheDocument();
        expect(join.mutate).toHaveBeenCalledWith({}, expect.anything());
    });

    it("asks for the board's password when it has one, and sends it", async () => {
        // given
        showPreview({ data: { ...open, requires_password: true } });
        const user = userEvent.setup();
        renderPage();
        await user.type(screen.getByLabelText("Your name"), "Robin");

        // when
        await user.click(screen.getByRole("button", { name: "Join board" }));

        // then
        expect(screen.getByText("Enter the board's password.")).toBeInTheDocument();
        expect(join.mutate).not.toHaveBeenCalled();

        // when
        await user.type(screen.getByLabelText("Board password"), "tide");
        await user.click(screen.getByRole("button", { name: "Join board" }));

        // then
        expect(join.mutate).toHaveBeenCalledWith({ display_name: "Robin", password: "tide" }, expect.anything());
    });

    it("puts a wrong password on the password field", () => {
        // given
        showPreview({ data: { ...open, requires_password: true } });
        mutations.useJoinBoardByLink.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 403,
                    message: "that is not the board's password",
                    code: "wrong_password",
                }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByText("That is not the board's password.")).toBeInTheDocument();
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it.each([
        ["board_full", 429, "This board already holds as many people as it can."],
        ["password_attempts", 429, "Too many wrong passwords for this board. Wait a few minutes, then try again."],
        ["link_disabled", 403, "The link to this board is switched off. Ask whoever shared it to switch it back on."],
    ])("explains a %s refusal of the join", (code, status, expected) => {
        // given
        mutations.useJoinBoardByLink.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status, message: "refused", code }) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(expected);
    });

    it("offers a sign in when the board started needing an account during the join", () => {
        // given
        mutations.useJoinBoardByLink.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 403,
                    message: "the board needs you to sign in",
                    code: "account_required",
                }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("link", { name: "Sign in to join" })).toHaveAttribute("href", "/login");
    });

    it("shows a failure it cannot explain in the server's own words", () => {
        // given
        mutations.useJoinBoardByLink.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 500, message: "internal error" }) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("internal error");
    });

    it("asks a visitor to sign in when the board is for accounts only", () => {
        // given
        showPreview({ data: { requires_password: false, requires_account: true, can_join: false } });

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "This board is only for people with an account. Sign in to join it.",
        );
        expect(screen.getByRole("link", { name: "Sign in to join" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Join board" })).not.toBeInTheDocument();
    });

    it("says a link that leads nowhere does, with no retry", () => {
        // given
        showPreview({ error: makeApiError({ status: 404, message: "board not found" }) });

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("This link does not lead to a board.");
        expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
    });

    it.each([
        ["removed", 403, "A facilitator removed you from this board, so you cannot join it again."],
        ["archived", 409, "This board is archived, so nobody can join it."],
        ["guests_not_allowed", 403, "This organisation does not let guests join its boards. Sign in to join it."],
    ])("explains a %s refusal of the preview", (code, status, expected) => {
        // given
        showPreview({ error: makeApiError({ status, message: "refused", code }) });

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(expected);
    });

    it("offers a retry when the preview failed for another reason", async () => {
        // given
        const preview = showPreview({ error: makeApiError({ status: 503, message: "service unavailable" }) });
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(preview.refetch).toHaveBeenCalledOnce();
    });

    it("lets a guest sign in instead, coming back to the link afterwards", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
    });
});
