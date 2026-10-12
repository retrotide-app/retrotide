import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardMembership } from "../../hooks/board/useBoardMembership";
import {
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeComment,
    makeMutationState,
    makeParticipantViewer,
    makePerson,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, CardView } from "../../types/api";
import { CardComments } from "./CardComments";

const mutations = vi.hoisted(() => ({
    useCreateComment: vi.fn(),
    useEditComment: vi.fn(),
    useDeleteComment: vi.fn(),
}));

const { useBoardMembership } = vi.hoisted(() => ({ useBoardMembership: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => mutations);
vi.mock("../../hooks/board/useBoardMembership", () => ({ useBoardMembership }));

const sam = makePerson({ user_id: "u1", display_name: "Sam" });

function membership(overrides: Partial<BoardMembership> = {}): BoardMembership {
    return { people: null, member: false, failure: null, retry: vi.fn(), ...overrides };
}

const discussing = makeBoardView({ stage: "discuss", cards_hidden: false });

const top = makeCard({ id: "k1" });
const under = makeCard({ id: "k2", parent_card_id: "k1" });

const mine = makeComment({ id: "m1", card_id: "k1", body: "Agreed" });
const theirs = makeComment({
    id: "m2",
    card_id: "k2",
    body: "Not sure",
    is_mine: undefined,
    author: { participant_id: "p-ali", display_name: "Ali" },
    updated_at: "2026-10-01T09:30:00Z",
});

function renderComments(
    snapshot: BoardSnapshot = makeBoardSnapshot({ comments: [mine, theirs] }),
    canComment = true,
    stack: CardView[] = [top, under],
) {
    return renderWithProviders(<CardComments snapshot={snapshot} stack={stack} canComment={canComment} writable />);
}

async function openThread(user: ReturnType<typeof userEvent.setup>, label: string) {
    await user.click(screen.getByRole("button", { name: label }));
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    useBoardMembership.mockReturnValue(membership());
});

describe("CardComments", () => {
    it("keeps the thread folded behind a count of the stack's comments until opened", async () => {
        // given
        const user = userEvent.setup();
        renderComments();

        // when
        const folded = screen.queryByRole("list", { name: "Comments" });
        await openThread(user, "2 comments");

        // then
        expect(folded).not.toBeInTheDocument();
        const thread = within(screen.getByRole("list", { name: "Comments" }));
        expect(thread.getAllByRole("listitem").map(item => item.textContent)).toEqual([
            expect.stringContaining("Agreed"),
            expect.stringContaining("Not sure"),
        ]);
        expect(thread.getAllByRole("listitem")[1]).toHaveTextContent("Ali · edited");
    });

    it("adds a comment to the stack's top card", async () => {
        // given
        const create = makeMutationState();
        mutations.useCreateComment.mockReturnValue(create);
        const user = userEvent.setup();
        renderComments(makeBoardSnapshot({ comments: [] }));

        // when
        await openThread(user, "Comment");
        await user.type(screen.getByRole("textbox", { name: "Comment text" }), "Good point{Enter}");

        // then
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k1", body: "Good point", mentions: [] },
            expect.anything(),
        );
    });

    it("mentions a member picked with @, where Enter picks the person rather than sending the comment", async () => {
        // given
        const create = makeMutationState();
        mutations.useCreateComment.mockReturnValue(create);
        useBoardMembership.mockReturnValue(membership({ people: [sam], member: true }));
        const user = userEvent.setup();
        renderComments(makeBoardSnapshot({ comments: [] }));

        // when
        await openThread(user, "Comment");
        const text = screen.getByRole("combobox", { name: "Comment text" });
        await user.type(text, "Over to @sa{Enter}");
        const picked = create.mutate.mock.calls.length;
        await user.type(text, "please{Enter}");

        // then
        expect(picked).toBe(0);
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k1", body: "Over to @Sam please", mentions: ["u1"] },
            expect.anything(),
        );
    });

    it("closes the @ list on Escape without leaving the comment", async () => {
        // given
        useBoardMembership.mockReturnValue(membership({ people: [sam], member: true }));
        const user = userEvent.setup();
        renderComments(makeBoardSnapshot({ comments: [] }));

        // when
        await openThread(user, "Comment");
        await user.type(screen.getByRole("combobox", { name: "Comment text" }), "Over to @sa");
        const listed = screen.getAllByRole("option").map(option => option.textContent);
        await user.keyboard("{Escape}");

        // then
        expect(listed).toEqual(["Sam"]);
        expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Comment text" })).toHaveValue("Over to @sa");
    });

    it("marks out each person a comment mentions in its text", async () => {
        // given
        const user = userEvent.setup();
        const named = makeComment({ id: "m3", card_id: "k1", body: "Ask @Sam", mentions: [sam] });
        renderComments(makeBoardSnapshot({ comments: [named] }));

        // when
        await openThread(user, "1 comment");

        // then
        const thread = within(screen.getByRole("list", { name: "Comments" }));
        expect(thread.getByText("@Sam")).toBeInTheDocument();
    });

    it("asks for some text before adding an empty comment", async () => {
        // given
        const create = makeMutationState();
        mutations.useCreateComment.mockReturnValue(create);
        const user = userEvent.setup();
        renderComments(makeBoardSnapshot({ comments: [] }));

        // when
        await openThread(user, "Comment");
        await user.click(screen.getByRole("button", { name: "Add comment" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Write something in the comment first.");
        expect(create.mutate).not.toHaveBeenCalled();
    });

    it("shows a refused comment in the form", async () => {
        // given
        mutations.useCreateComment.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "comments are switched off", code: "comments_disabled" }),
            }),
        );
        const user = userEvent.setup();
        renderComments(makeBoardSnapshot({ comments: [] }));

        // when
        await openThread(user, "Comment");

        // then
        expect(screen.getByRole("form", { name: "Add a comment" })).toHaveTextContent("comments are switched off");
    });

    it("edits the viewer's own comment in place", async () => {
        // given
        const edit = makeMutationState();
        mutations.useEditComment.mockReturnValue(edit);
        const user = userEvent.setup();
        renderComments(
            makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer(), comments: [mine, theirs] }),
        );

        // when
        await openThread(user, "2 comments");
        const own = within(screen.getByRole("list", { name: "Comments" })).getAllByRole("listitem")[0];
        await user.click(within(own).getByRole("button", { name: "Comment options" }));
        await user.click(screen.getByRole("menuitem", { name: "Edit comment" }));
        const text = screen.getByRole("form", { name: "Edit comment" });
        await user.clear(within(text).getByRole("textbox", { name: "Comment text" }));
        await user.type(within(text).getByRole("textbox", { name: "Comment text" }), "Agreed, mostly");
        await user.click(within(text).getByRole("button", { name: "Save comment" }));

        // then
        expect(edit.mutate).toHaveBeenCalledExactlyOnceWith(
            { commentId: "m1", body: "Agreed, mostly", mentions: [] },
            expect.anything(),
        );
    });

    it("keeps the people an edited comment still names", async () => {
        // given
        const edit = makeMutationState();
        mutations.useEditComment.mockReturnValue(edit);
        useBoardMembership.mockReturnValue(membership({ people: [sam], member: true }));
        const named = makeComment({ id: "m4", card_id: "k1", body: "Ask @Sam", mentions: [sam] });
        const user = userEvent.setup();
        renderComments(makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer(), comments: [named] }));

        // when
        await openThread(user, "1 comment");
        await user.click(screen.getByRole("button", { name: "Comment options" }));
        await user.click(screen.getByRole("menuitem", { name: "Edit comment" }));
        const form = within(screen.getByRole("form", { name: "Edit comment" }));
        await user.type(form.getByRole("combobox", { name: "Comment text" }), " today");
        await user.click(form.getByRole("button", { name: "Save comment" }));

        // then
        expect(edit.mutate).toHaveBeenCalledExactlyOnceWith(
            { commentId: "m4", body: "Ask @Sam today", mentions: ["u1"] },
            expect.anything(),
        );
    });

    it("lets a participant touch only their own comments", async () => {
        // given
        const user = userEvent.setup();
        renderComments(
            makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer(), comments: [mine, theirs] }),
        );

        // when
        await openThread(user, "2 comments");

        // then
        const [own, other] = within(screen.getByRole("list", { name: "Comments" })).getAllByRole("listitem");
        expect(within(own).getByRole("button", { name: "Comment options" })).toBeInTheDocument();
        expect(within(other).queryByRole("button", { name: "Comment options" })).not.toBeInTheDocument();
    });

    it.each(["write", "vote", "actions"] as const)(
        "keeps the thread readable but leaves even the participant's own comment alone in %s",
        async stage => {
            // given
            const user = userEvent.setup();
            renderComments(
                makeBoardSnapshot({
                    board: makeBoardView({ stage, cards_hidden: false }),
                    viewer: makeParticipantViewer(),
                    comments: [mine, theirs],
                }),
                false,
            );

            // when
            await openThread(user, "2 comments");

            // then
            const thread = screen.getByRole("list", { name: "Comments" });
            expect(thread).toHaveTextContent("Agreed");
            expect(within(thread).queryByRole("button", { name: "Comment options" })).not.toBeInTheDocument();
            expect(screen.queryByRole("form", { name: "Add a comment" })).not.toBeInTheDocument();
        },
    );

    it("lets a facilitator delete someone else's comment", async () => {
        // given
        const removal = makeMutationState();
        mutations.useDeleteComment.mockReturnValue(removal);
        const user = userEvent.setup();
        renderComments();

        // when
        await openThread(user, "2 comments");
        const other = within(screen.getByRole("list", { name: "Comments" })).getAllByRole("listitem")[1];
        await user.click(within(other).getByRole("button", { name: "Comment options" }));
        const options = within(screen.getByRole("menu"))
            .getAllByRole("menuitem")
            .map(item => item.textContent);
        await user.click(screen.getByRole("menuitem", { name: "Delete comment" }));

        // then
        expect(options).toEqual(["Delete comment"]);
        expect(removal.mutate).toHaveBeenCalledExactlyOnceWith({ commentId: "m2" });
    });

    it("shows the thread but no way to add to it when the viewer may not comment", async () => {
        // given
        const user = userEvent.setup();
        renderComments(
            makeBoardSnapshot({ comments: [mine], settings: makeBoardSettings({ comments_enabled: false }) }),
            false,
        );

        // when
        await openThread(user, "1 comment");

        // then
        expect(screen.getByRole("list", { name: "Comments" })).toBeInTheDocument();
        expect(screen.queryByRole("form", { name: "Add a comment" })).not.toBeInTheDocument();
    });

    it("shows nothing at all when there are no comments and the viewer may not comment", () => {
        // when
        renderComments(makeBoardSnapshot({ comments: [] }), false);

        // then
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });
});
