import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { QueryState } from "../../hooks/queries/queryState";
import {
    makeApiError,
    makeCardMention,
    makeCommentMention,
    makeMention,
    makeQueryState,
    makeSessionUser,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { MentionListResponse } from "../../types/api";
import { MentionsSection } from "./MentionsSection";

const { useMentions } = vi.hoisted(() => ({ useMentions: vi.fn() }));

vi.mock("../../hooks/queries/actionItems", () => ({ useMentions }));

function showMentions(state: Partial<QueryState<MentionListResponse>>) {
    useMentions.mockReturnValue(makeQueryState<MentionListResponse>(state));
}

function renderSection() {
    return renderWithProviders(<MentionsSection />, { user: makeSessionUser() });
}

beforeEach(() => {
    vi.clearAllMocks();
});

describe("MentionsSection", () => {
    it("lists each action item the user was mentioned in, in the order the server gives, with a link to its board", () => {
        // given
        showMentions({
            data: {
                mentions: [
                    makeMention({ action_item_id: "i2", title: "Ask @Sam about the release", due_on: "2026-10-20" }),
                    makeMention({
                        action_item_id: "i1",
                        title: "Pair with @Sam",
                        status: "done",
                        org_slug: "lighthouse",
                        org_name: "Lighthouse",
                        board_id: "b9",
                        board_title: "Planning",
                    }),
                ],
            },
        });

        // when
        renderSection();

        // then
        const section = within(screen.getByRole("region", { name: "Mentions" }));
        const rows = section.getAllByRole("listitem");
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent("Ask @Sam about the release");
        expect(rows[1]).toHaveTextContent("Pair with @Sam");
        expect(within(rows[0]).getByRole("link", { name: "Sprint 12" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/00000000-0000-4000-8000-0000000000b1",
        );
        expect(within(rows[0]).getByText(/Due 20 Oct 2026/)).toBeInTheDocument();
        expect(within(rows[1]).getByRole("link", { name: "Planning" })).toHaveAttribute(
            "href",
            "/o/lighthouse/boards/b9",
        );
        expect(within(rows[1]).getByText("Done")).toBeInTheDocument();
    });

    it("lists card and comment mentions beside action items, each with its kind and a link to its board", () => {
        // given
        showMentions({
            data: {
                mentions: [
                    makeCommentMention({ comment_id: "m1", body: "Agreed, @Sam can pair on it" }),
                    makeCardMention({ card_id: "k1", body: "Thanks @Sam", board_id: "b9", board_title: "Planning" }),
                    makeMention({ action_item_id: "i1", title: "Pair with @Sam" }),
                ],
            },
        });

        // when
        renderSection();

        // then
        const rows = within(screen.getByRole("region", { name: "Mentions" })).getAllByRole("listitem");
        expect(rows.map(row => row.textContent)).toEqual([
            expect.stringContaining("Agreed, @Sam can pair on it"),
            expect.stringContaining("Thanks @Sam"),
            expect.stringContaining("Pair with @Sam"),
        ]);
        expect(rows[0]).toHaveTextContent("Comment · Harbour · Sprint 12");
        expect(rows[1]).toHaveTextContent("Card · Harbour · Planning");
        expect(rows[2]).toHaveTextContent("Action item · Harbour · Sprint 12");
        expect(within(rows[1]).getByRole("link", { name: "Planning" })).toHaveAttribute("href", "/o/harbour/boards/b9");
        expect(within(rows[0]).queryByText("Open")).not.toBeInTheDocument();
        expect(within(rows[2]).getByText("Open")).toBeInTheDocument();
    });

    it("names no board when the board is gone", () => {
        // given
        showMentions({
            data: { mentions: [makeMention({ board_id: undefined, board_title: undefined })] },
        });

        // when
        renderSection();

        // then
        expect(screen.getByText(/Board deleted/)).toBeInTheDocument();
        expect(screen.queryByRole("link")).not.toBeInTheDocument();
    });

    it("says so when nobody has mentioned the user", () => {
        // given
        showMentions({ data: { mentions: [] } });

        // when
        renderSection();

        // then
        expect(screen.getByText("Nobody has mentioned you yet.")).toBeInTheDocument();
    });

    it("says why the mentions failed to load, and tries again", async () => {
        // given
        const refetch = vi.fn().mockResolvedValue(undefined);
        showMentions({ error: makeApiError({ status: 503, message: "service unavailable" }), refetch });
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(refetch).toHaveBeenCalledTimes(1);
    });
});
