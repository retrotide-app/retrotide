import { describe, expect, it } from "vitest";
import {
    makeActionItem,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeBoardViewer,
    makeCard,
    makeColumn,
    makeComment,
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import {
    actionItemActions,
    boardActions,
    boardInteractions,
    boardManagementActions,
    canExportBoard,
    canRemoveActionItem,
    cardActions,
    cardInteractions,
    cardPolishActions,
    columnCardsHidden,
    columnHidden,
    commentActions,
    isReadOnly,
    NO_ACTION_ITEM_ACTIONS,
    surveyActions,
} from "./capabilities";

const archived = makeBoardView({ archived_at: "2026-10-02T09:00:00Z" });

const writing = makeBoardView({ cards_hidden: false, stage: "write" });

const voting = makeBoardView({ cards_hidden: false, stage: "vote" });

const discussing = makeBoardView({ cards_hidden: false, stage: "discuss" });

const agreeingActions = makeBoardView({ cards_hidden: false, stage: "actions" });

describe("canExportBoard", () => {
    it("lets a viewer with board.export export, ended and archived boards included", () => {
        // given
        const snapshots = [
            makeBoardSnapshot({ viewer: makeParticipantViewer() }),
            makeBoardSnapshot({ board: makeBoardView({ closed: true, stage: "end" }) }),
            makeBoardSnapshot({ board: archived }),
        ];

        // when
        const exports = snapshots.map(canExportBoard);

        // then
        expect(exports).toEqual([true, true, true]);
    });

    it("refuses a viewer without board.export", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeParticipantViewer({ capabilities: ["board.view"] }) });

        // when
        const exports = canExportBoard(snapshot);

        // then
        expect(exports).toBe(false);
    });
});

describe("isReadOnly", () => {
    it("tells an archived board from an open one", () => {
        // when
        const readOnly = [
            isReadOnly(archived),
            isReadOnly(makeBoardView()),
            isReadOnly(makeBoardView({ archived_at: "" })),
        ];

        // then
        expect(readOnly).toEqual([true, false, false]);
    });
});

describe("boardActions", () => {
    it("lets a facilitator do everything on an open board in Write", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeBoardViewer() });

        // when
        const actions = boardActions(snapshot);

        // then
        expect(actions).toEqual({
            addCards: true,
            moveCards: true,
            manageColumns: true,
            removeAnyCard: true,
            describeBoard: true,
            removeColumns: true,
            controlTimer: true,
            spotlight: true,
        });
    });

    it("lets a participant add and move cards in Write but not run the board", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeParticipantViewer() });

        // when
        const actions = boardActions(snapshot);

        // then
        expect(actions).toEqual({
            addCards: true,
            moveCards: true,
            manageColumns: false,
            removeAnyCard: false,
            describeBoard: false,
            removeColumns: false,
            controlTimer: false,
            spotlight: false,
        });
    });

    it("lets a seated participant shape columns and run the timer when anyone can edit, and nothing more", () => {
        // given
        const settings = makeBoardSettings({ anyone_can_edit: true });
        const seated = makeBoardSnapshot({ settings, viewer: makeParticipantViewer() });
        const unseated = makeBoardSnapshot({ settings, viewer: makeParticipantViewer({ participant_id: null }) });
        const locked = makeBoardSnapshot({ settings, viewer: makeParticipantViewer({ capabilities: ["board.view"] }) });

        // when
        const actions = boardActions(seated);

        // then
        expect(actions.manageColumns).toBe(true);
        expect(actions.controlTimer).toBe(true);
        expect(actions.removeColumns).toBe(false);
        expect(actions.spotlight).toBe(false);
        expect(actions.removeAnyCard).toBe(false);
        expect(boardActions(unseated).manageColumns).toBe(false);
        expect(boardActions(unseated).controlTimer).toBe(false);
        expect(boardActions(locked).manageColumns).toBe(false);
        expect(boardActions(locked).controlTimer).toBe(false);
    });

    it("keeps a viewer with no participant seat from adding cards", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeParticipantViewer({ participant_id: null }) });

        // when
        const actions = boardActions(snapshot);

        // then
        expect(actions.addCards).toBe(false);
        expect(actions.moveCards).toBe(true);
    });

    it.each([voting, discussing, agreeingActions])(
        "lets nobody add or move a card in $stage, while a facilitator still runs the columns and removes cards",
        board => {
            // given
            const facilitator = makeBoardSnapshot({ board, viewer: makeBoardViewer() });
            const participant = makeBoardSnapshot({ board, viewer: makeParticipantViewer() });

            // then
            expect(boardActions(facilitator)).toEqual({
                addCards: false,
                moveCards: false,
                manageColumns: true,
                removeAnyCard: true,
                describeBoard: true,
                removeColumns: true,
                controlTimer: true,
                spotlight: true,
            });
            expect(boardActions(participant)).toEqual({
                addCards: false,
                moveCards: false,
                manageColumns: false,
                removeAnyCard: false,
                describeBoard: false,
                removeColumns: false,
                controlTimer: false,
                spotlight: false,
            });
        },
    );

    it("reads the capabilities the server sent, not the role's name", () => {
        // given
        const snapshot = makeBoardSnapshot({
            viewer: makeBoardViewer({ role: "facilitator", capabilities: ["board.view"] }),
        });

        // when
        const actions = boardActions(snapshot);

        // then
        expect(Object.values(actions)).toEqual([false, false, false, false, false, false, false, false]);
    });

    it("allows nothing on an archived board, whatever the viewer holds", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: archived, viewer: makeBoardViewer() });

        // when
        const actions = boardActions(snapshot);

        // then
        expect(Object.values(actions)).toEqual([false, false, false, false, false, false, false, false]);
    });
});

describe("cardActions", () => {
    it("lets the author edit, recolour, remove and move their own card in Write", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeParticipantViewer() });

        // when
        const actions = cardActions(snapshot, makeCard({ is_mine: true }));

        // then
        expect(actions).toEqual({ edit: true, recolour: true, remove: true, move: true });
    });

    it("lets a participant do nothing to someone else's card, not even move it", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: writing, viewer: makeParticipantViewer() });
        const theirs = makeCard({ is_mine: undefined, hidden: false });

        // when
        const actions = cardActions(snapshot, theirs);

        // then
        expect(actions).toEqual({ edit: false, recolour: false, remove: false, move: false });
    });

    it("lets a facilitator remove and move someone else's card in Write but never edit it", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: writing, viewer: makeBoardViewer() });
        const theirs = makeCard({ is_mine: undefined, hidden: false });

        // when
        const actions = cardActions(snapshot, theirs);

        // then
        expect(actions).toEqual({ edit: false, recolour: false, remove: true, move: true });
    });

    it.each([voting, discussing, agreeingActions])(
        "leaves the author nothing to do to their own card in $stage, and a facilitator only removal",
        board => {
            // given
            const own = makeCard({ is_mine: true, hidden: false });
            const participant = makeBoardSnapshot({ board, viewer: makeParticipantViewer() });
            const facilitator = makeBoardSnapshot({ board, viewer: makeBoardViewer() });

            // then
            expect(cardActions(participant, own)).toEqual({ edit: false, recolour: false, remove: false, move: false });
            expect(cardActions(facilitator, own)).toEqual({ edit: false, recolour: false, remove: true, move: false });
        },
    );

    it("allows nothing on a card of an archived board, even the author's own", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: archived, viewer: makeBoardViewer() });

        // when
        const actions = cardActions(snapshot, makeCard({ is_mine: true }));

        // then
        expect(actions).toEqual({ edit: false, recolour: false, remove: false, move: false });
    });
});

describe("boardInteractions", () => {
    it("lets a seated participant only vote in Vote", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: voting, viewer: makeParticipantViewer() });

        // when
        const interactions = boardInteractions(snapshot);

        // then
        expect(interactions).toEqual({ vote: true, comment: false, mergeCards: false, facilitate: false });
    });

    it("lets a seated participant comment and merge in Discuss, but no longer vote", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer() });

        // when
        const interactions = boardInteractions(snapshot);

        // then
        expect(interactions).toEqual({ vote: false, comment: true, mergeCards: true, facilitate: false });
    });

    it.each([writing, agreeingActions])("lets nobody vote, comment or merge in $stage", board => {
        // given
        const facilitator = makeBoardSnapshot({ board, viewer: makeBoardViewer() });
        const participant = makeBoardSnapshot({ board, viewer: makeParticipantViewer() });

        // then
        expect(boardInteractions(facilitator)).toEqual({
            vote: false,
            comment: false,
            mergeCards: false,
            facilitate: true,
        });
        expect(boardInteractions(participant)).toEqual({
            vote: false,
            comment: false,
            mergeCards: false,
            facilitate: false,
        });
    });

    it("binds a facilitator to the stage too, so there is no voting in Discuss and no comment in Vote", () => {
        // given
        const inVote = makeBoardSnapshot({ board: voting, viewer: makeBoardViewer() });
        const inDiscuss = makeBoardSnapshot({ board: discussing, viewer: makeBoardViewer() });

        // then
        expect(boardInteractions(inVote)).toEqual({ vote: true, comment: false, mergeCards: false, facilitate: true });
        expect(boardInteractions(inDiscuss)).toEqual({
            vote: false,
            comment: true,
            mergeCards: true,
            facilitate: true,
        });
    });

    it("follows the facilitator's switches for voting, comments and merging inside their stage", () => {
        // given
        const settings = makeBoardSettings({ voting_open: false, comments_enabled: false, merging_enabled: false });
        const inVote = makeBoardSnapshot({ board: voting, settings });
        const inDiscuss = makeBoardSnapshot({ board: discussing, settings });

        // then
        expect(boardInteractions(inVote).vote).toBe(false);
        expect(boardInteractions(inDiscuss)).toEqual({
            vote: false,
            comment: false,
            mergeCards: false,
            facilitate: true,
        });
    });

    it("refuses merging while cards are hidden", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: makeBoardView({ cards_hidden: true }) });

        // then
        expect(boardInteractions(snapshot).mergeCards).toBe(false);
    });

    it("keeps a viewer with no seat from voting or commenting", () => {
        // given
        const viewer = makeParticipantViewer({ participant_id: null });

        // then
        expect(boardInteractions(makeBoardSnapshot({ board: voting, viewer })).vote).toBe(false);
        expect(boardInteractions(makeBoardSnapshot({ board: discussing, viewer })).comment).toBe(false);
    });

    it("allows nothing on an archived board", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: archived });

        // then
        expect(boardInteractions(snapshot)).toEqual({
            vote: false,
            comment: false,
            mergeCards: false,
            facilitate: false,
        });
    });
});

describe("cardInteractions", () => {
    const top = makeCard({ id: "k1" });
    const under = makeCard({ id: "k2", parent_card_id: "k1" });

    it("lets a top card be merged into another, and a merged card be taken back out, in Discuss", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: discussing, cards: [top, under] });

        // then
        expect(cardInteractions(snapshot, top)).toMatchObject({ merge: true, unmerge: false });
        expect(cardInteractions(snapshot, under)).toMatchObject({ merge: false, unmerge: true });
    });

    it("still lets a stack be taken apart once merging is switched off", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: discussing,
            settings: makeBoardSettings({ merging_enabled: false }),
            cards: [top, under],
        });

        // then
        expect(cardInteractions(snapshot, under).unmerge).toBe(true);
        expect(cardInteractions(snapshot, top).merge).toBe(false);
    });

    it.each([writing, voting, agreeingActions])("merges and unmerges nothing in $stage", board => {
        // given
        const snapshot = makeBoardSnapshot({ board, cards: [top, under] });

        // then
        expect(cardInteractions(snapshot, top).merge).toBe(false);
        expect(cardInteractions(snapshot, under).unmerge).toBe(false);
    });

    it("votes only in Vote and comments only in Discuss", () => {
        // given
        const inVote = makeBoardSnapshot({ board: voting, cards: [top] });
        const inDiscuss = makeBoardSnapshot({ board: discussing, cards: [top] });

        // then
        expect(cardInteractions(inVote, top)).toMatchObject({ vote: true, comment: false });
        expect(cardInteractions(inDiscuss, top)).toMatchObject({ vote: false, comment: true });
    });
});

describe("a hidden column", () => {
    const shut = makeColumn({ id: "c-shut", hidden: true });
    const open = makeColumn({ id: "c-open" });
    const frozen = makeCard({ id: "k-frozen", column_id: "c-shut" });

    it("is frozen for a facilitator: no card action or interaction is offered in any stage", () => {
        // given
        const stages = [writing, voting, discussing].map(board =>
            makeBoardSnapshot({ board, columns: [open, shut], cards: [frozen] }),
        );

        // then
        for (const snapshot of stages) {
            expect(cardActions(snapshot, frozen)).toEqual({ edit: false, recolour: false, remove: false, move: false });
            expect(cardInteractions(snapshot, frozen)).toEqual({
                vote: false,
                comment: false,
                merge: false,
                unmerge: false,
            });
        }
    });

    it("is told apart from an open column", () => {
        // given
        const snapshot = makeBoardSnapshot({ columns: [open, shut] });

        // then
        expect(columnHidden(snapshot, "c-shut")).toBe(true);
        expect(columnHidden(snapshot, "c-open")).toBe(false);
        expect(columnHidden(snapshot, "c-gone")).toBe(false);
    });
});

describe("columnCardsHidden", () => {
    it("hides a column's cards while the board hides cards, unless that column was revealed", () => {
        // given
        const revealed = makeColumn({ id: "c-revealed", cards_revealed: true });
        const covered = makeColumn({ id: "c-covered" });
        const hiding = makeBoardSnapshot({
            board: makeBoardView({ cards_hidden: true }),
            columns: [revealed, covered],
        });
        const showing = makeBoardSnapshot({ board: writing, columns: [revealed, covered] });

        // then
        expect(columnCardsHidden(hiding, "c-revealed")).toBe(false);
        expect(columnCardsHidden(hiding, "c-covered")).toBe(true);
        expect(columnCardsHidden(showing, "c-covered")).toBe(false);
    });
});

describe("commentActions", () => {
    it("lets the writer edit and delete their own comment in Discuss while comments are on", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer() });

        // then
        expect(commentActions(snapshot, makeComment({ is_mine: true }))).toEqual({ edit: true, remove: true });
    });

    it("still lets the writer delete their own comment once comments are switched off", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: discussing,
            viewer: makeParticipantViewer(),
            settings: makeBoardSettings({ comments_enabled: false }),
        });

        // then
        expect(commentActions(snapshot, makeComment({ is_mine: true }))).toEqual({ edit: false, remove: true });
    });

    it.each([writing, voting, agreeingActions])(
        "leaves the writer's own comment read-only in $stage, while a facilitator can still delete it",
        board => {
            // given
            const participant = makeBoardSnapshot({ board, viewer: makeParticipantViewer() });
            const facilitator = makeBoardSnapshot({ board, viewer: makeBoardViewer() });
            const own = makeComment({ is_mine: true });

            // then
            expect(commentActions(participant, own)).toEqual({ edit: false, remove: false });
            expect(commentActions(facilitator, own)).toEqual({ edit: false, remove: true });
        },
    );

    it("lets a facilitator delete someone else's comment but never edit it", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: discussing, viewer: makeBoardViewer() });

        // then
        expect(commentActions(snapshot, makeComment({ is_mine: undefined }))).toEqual({ edit: false, remove: true });
    });

    it("lets a participant do nothing to someone else's comment", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer() });

        // then
        expect(commentActions(snapshot, makeComment({ is_mine: undefined }))).toEqual({ edit: false, remove: false });
    });
});

describe("an ended retro", () => {
    const ended = makeBoardView({ closed: true, cards_hidden: false, stage: "end" });

    it("is read-only, like an archived board", () => {
        // then
        expect(isReadOnly(ended)).toBe(true);
    });

    it("allows nobody to change anything, facilitators included", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: ended, viewer: makeBoardViewer() });

        // when
        const actions = boardActions(snapshot);
        const interactions = boardInteractions(snapshot);

        // then
        expect(Object.values(actions)).toEqual([false, false, false, false, false, false, false, false]);
        expect(interactions).toEqual({ vote: false, comment: false, mergeCards: false, facilitate: false });
        expect(cardActions(snapshot, makeCard())).toEqual({ edit: false, recolour: false, remove: false, move: false });
    });
});

describe("the locks", () => {
    const under = makeCard({ id: "under", parent_card_id: "top", hidden: false });
    const stack = [makeCard({ id: "top", hidden: false }), under];

    it("keeps a participant from adding, editing, recolouring or deleting their own card while cards are locked", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: writing,
            settings: makeBoardSettings({ cards_locked: true }),
            viewer: makeParticipantViewer(),
        });

        // when
        const actions = boardActions(snapshot);
        const own = cardActions(snapshot, makeCard({ hidden: false }));

        // then
        expect(actions.addCards).toBe(false);
        expect(actions.moveCards).toBe(true);
        expect(own).toEqual({ edit: false, recolour: false, remove: false, move: true });
    });

    it("keeps a participant from moving in Write, and from merging or unmerging in Discuss, while moves are locked", () => {
        // given
        const settings = makeBoardSettings({ moves_locked: true });
        const inWrite = makeBoardSnapshot({ board: writing, settings, viewer: makeParticipantViewer(), cards: stack });
        const inDiscuss = makeBoardSnapshot({
            board: discussing,
            settings,
            viewer: makeParticipantViewer(),
            cards: stack,
        });

        // when
        const actions = boardActions(inWrite);

        // then
        expect(actions.moveCards).toBe(false);
        expect(actions.addCards).toBe(true);
        expect(cardActions(inWrite, under).move).toBe(false);
        expect(boardInteractions(inDiscuss).mergeCards).toBe(false);
        expect(cardInteractions(inDiscuss, under).unmerge).toBe(false);
    });

    it("leaves a facilitator free of both locks", () => {
        // given
        const settings = makeBoardSettings({ cards_locked: true, moves_locked: true });
        const inWrite = makeBoardSnapshot({ board: writing, settings, viewer: makeBoardViewer(), cards: stack });
        const inDiscuss = makeBoardSnapshot({ board: discussing, settings, viewer: makeBoardViewer(), cards: stack });

        // when
        const actions = boardActions(inWrite);

        // then
        expect(actions.addCards).toBe(true);
        expect(actions.moveCards).toBe(true);
        expect(cardActions(inWrite, under)).toEqual({ edit: true, recolour: true, remove: true, move: true });
        expect(boardInteractions(inDiscuss).mergeCards).toBe(true);
        expect(cardInteractions(inDiscuss, under).unmerge).toBe(true);
    });

    it("follows the capabilities the server narrows while the whole board is locked", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: writing,
            settings: makeBoardSettings({ board_locked: true }),
            viewer: makeParticipantViewer({ capabilities: ["board.view", "board.export"] }),
        });

        // when
        const actions = boardActions(snapshot);

        // then
        expect(actions.addCards).toBe(false);
        expect(actions.moveCards).toBe(false);
        expect(boardInteractions({ ...snapshot, board: voting }).vote).toBe(false);
    });

    it("still lets a facilitator move in Write, and merge and unmerge in Discuss, while the whole board is locked", () => {
        // given
        const settings = makeBoardSettings({ board_locked: true });
        const viewer = makeBoardViewer({
            capabilities: ["board.view", "board.facilitate", "board.manage", "board.export"],
        });
        const inWrite = makeBoardSnapshot({ board: writing, settings, viewer, cards: stack });
        const inDiscuss = makeBoardSnapshot({ board: discussing, settings, viewer, cards: stack });

        // then
        expect(boardActions(inWrite).moveCards).toBe(true);
        expect(cardActions(inWrite, under).move).toBe(true);
        expect(boardInteractions(inDiscuss).mergeCards).toBe(true);
        expect(cardInteractions(inDiscuss, under).unmerge).toBe(true);
    });
});

describe("actionItemActions", () => {
    const ended = makeBoardView({ cards_hidden: false, stage: "end", closed: true });

    it("lets a seated participant add, edit and mark items in the Actions stage, and name people when a member", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: agreeingActions, viewer: makeParticipantViewer() });

        // when
        const actions = actionItemActions(snapshot, true);

        // then
        expect(actions).toEqual({
            add: true,
            edit: true,
            mark: true,
            remove: false,
            removeOwn: true,
            name: true,
            review: true,
        });
    });

    it("never lets a guest or a link-only account name anyone or review a previous action", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: agreeingActions, viewer: makeParticipantViewer() });

        // when
        const actions = actionItemActions(snapshot, false);

        // then
        expect(actions.add).toBe(true);
        expect(actions.name).toBe(false);
        expect(actions.review).toBe(false);
    });

    it.each([writing, voting, discussing])("keeps items read-only outside the Actions stage", board => {
        // given
        const snapshot = makeBoardSnapshot({ board, viewer: makeParticipantViewer() });

        // when
        const actions = actionItemActions(snapshot, true);

        // then
        expect(actions.add).toBe(false);
        expect(actions.edit).toBe(false);
        expect(actions.mark).toBe(false);
        expect(actions.name).toBe(false);
    });

    it("lets a facilitator delete an item in any stage before End", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: discussing });

        // then
        expect(actionItemActions(snapshot, true).remove).toBe(true);
        expect(actionItemActions({ ...snapshot, board: ended }, true).remove).toBe(false);
    });

    it("lets a viewer with no seat mark items but never add or edit them", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: agreeingActions,
            viewer: makeParticipantViewer({ participant_id: null }),
        });

        // when
        const actions = actionItemActions(snapshot, true);

        // then
        expect(actions.add).toBe(false);
        expect(actions.edit).toBe(false);
        expect(actions.mark).toBe(true);
    });

    it("lets only an organisation member mark items after End, and nothing else", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: ended, viewer: makeParticipantViewer() });

        // then
        expect(actionItemActions(snapshot, true)).toEqual({
            add: false,
            edit: false,
            mark: true,
            remove: false,
            removeOwn: false,
            name: false,
            review: false,
        });
        expect(actionItemActions(snapshot, false).mark).toBe(false);
    });

    it("lets the creator delete their own item in the Actions stage, and a facilitator any item", () => {
        // given
        const participant = makeBoardSnapshot({ board: agreeingActions, viewer: makeParticipantViewer() });
        const facilitator = makeBoardSnapshot({ board: agreeingActions });
        const own = makeActionItem({ is_mine: true });
        const theirs = makeActionItem();

        // when
        const participantActions = actionItemActions(participant, true);
        const facilitatorActions = actionItemActions(facilitator, true);

        // then
        expect(canRemoveActionItem(participantActions, own)).toBe(true);
        expect(canRemoveActionItem(participantActions, theirs)).toBe(false);
        expect(canRemoveActionItem(facilitatorActions, theirs)).toBe(true);
    });

    it.each([writing, voting, discussing])("keeps a creator from deleting their own item outside Actions", board => {
        // given
        const snapshot = makeBoardSnapshot({ board, viewer: makeParticipantViewer() });

        // when
        const actions = actionItemActions(snapshot, true);

        // then
        expect(canRemoveActionItem(actions, makeActionItem({ is_mine: true }))).toBe(false);
    });

    it("offers no delete at all when the board is not writable", () => {
        // then
        expect(canRemoveActionItem(NO_ACTION_ITEM_ACTIONS, makeActionItem({ is_mine: true }))).toBe(false);
    });

    it("keeps an archived board's items read-only for everyone", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ stage: "end", closed: true, archived_at: "2026-10-02T09:00:00Z" }),
        });

        // then
        expect(actionItemActions(snapshot, true).mark).toBe(false);
    });

    it("reviews previous actions on team boards only, in any stage before End", () => {
        // given
        const team = makeBoardSnapshot({ board: writing, viewer: makeParticipantViewer() });
        const organisation = makeBoardSnapshot({
            board: makeBoardView({ stage: "write", team_id: undefined }),
            viewer: makeParticipantViewer(),
        });

        // then
        expect(actionItemActions(team, true).review).toBe(true);
        expect(actionItemActions(organisation, true).review).toBe(false);
        expect(actionItemActions({ ...team, board: ended }, true).review).toBe(false);
    });
});

describe("cardPolishActions", () => {
    const shown = makeCard({ hidden: false, is_mine: undefined });

    it("lets a seated participant react to a card everyone sees, and only a facilitator pin it", () => {
        // given
        const participant = makeBoardSnapshot({ board: voting, viewer: makeParticipantViewer() });
        const facilitator = makeBoardSnapshot({ board: voting });

        // then
        expect(cardPolishActions(participant, shown)).toEqual({ react: true, pin: false, setImage: false });
        expect(cardPolishActions(facilitator, shown)).toEqual({ react: true, pin: true, setImage: false });
    });

    it("refuses reactions and pins on a hidden card and in a hidden column", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: writing, columns: [makeColumn({ hidden: true })] });

        // then
        expect(cardPolishActions(snapshot, makeCard({ hidden: true }))).toMatchObject({ react: false, pin: false });
        expect(cardPolishActions(snapshot, shown)).toMatchObject({ react: false, pin: false });
    });

    it("refuses a viewer with no seat and a read-only board", () => {
        // given
        const unseated = makeBoardSnapshot({ board: voting, viewer: makeParticipantViewer({ participant_id: null }) });
        const readOnly = makeBoardSnapshot({ board: archived });

        // then
        expect(cardPolishActions(unseated, shown).react).toBe(false);
        expect(cardPolishActions(readOnly, shown)).toEqual({ react: false, pin: false, setImage: false });
    });

    it("lets an organisation member attach an image to a card they may edit, and never a guest", () => {
        // given
        const member = makeBoardSnapshot({ board: writing, viewer: makeParticipantViewer() });
        const guest = makeBoardSnapshot({
            board: writing,
            viewer: makeParticipantViewer({ capabilities: ["board.view", "board.participate"] }),
        });
        const own = makeCard({ hidden: true });

        // then
        expect(cardPolishActions(member, own).setImage).toBe(true);
        expect(cardPolishActions(guest, own).setImage).toBe(false);
        expect(cardPolishActions(member, shown).setImage).toBe(false);
        expect(cardPolishActions({ ...member, board: voting }, own).setImage).toBe(false);
    });
});

describe("surveyActions", () => {
    it("lets a facilitator manage surveys and any seated participant answer them", () => {
        // given
        const facilitator = makeBoardSnapshot({ board: discussing });
        const participant = makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer() });
        const unseated = makeBoardSnapshot({
            board: discussing,
            viewer: makeParticipantViewer({ participant_id: null }),
        });

        // then
        expect(surveyActions(facilitator)).toEqual({ manage: true, respond: true });
        expect(surveyActions(participant)).toEqual({ manage: false, respond: true });
        expect(surveyActions(unseated)).toEqual({ manage: false, respond: false });
    });

    it("refuses both on an ended or archived board", () => {
        // then
        expect(surveyActions(makeBoardSnapshot({ board: makeBoardView({ closed: true, stage: "end" }) }))).toEqual({
            manage: false,
            respond: false,
        });
        expect(surveyActions(makeBoardSnapshot({ board: archived }))).toEqual({ manage: false, respond: false });
    });
});

describe("boardManagementActions", () => {
    it("gives a facilitator every action in the Write stage", () => {
        // when
        const actions = boardManagementActions(makeBoardSnapshot({ board: writing }));

        // then
        expect(actions).toEqual({ archive: true, remove: true, move: true, importCards: true, background: true });
    });

    it("keeps archive, delete and move on an archived or ended board, and drops import and the background", () => {
        // given
        const ended = makeBoardSnapshot({ board: makeBoardView({ closed: true, stage: "end" }) });

        // then
        expect(boardManagementActions(makeBoardSnapshot({ board: archived }))).toEqual({
            archive: true,
            remove: true,
            move: true,
            importCards: false,
            background: false,
        });
        expect(boardManagementActions(ended)).toMatchObject({ archive: true, importCards: false, background: false });
    });

    it("imports only in the Write stage", () => {
        // then
        expect(boardManagementActions(makeBoardSnapshot({ board: voting })).importCards).toBe(false);
    });

    it("gives a participant nothing", () => {
        // when
        const actions = boardManagementActions(makeBoardSnapshot({ board: writing, viewer: makeParticipantViewer() }));

        // then
        expect(actions).toEqual({ archive: false, remove: false, move: false, importCards: false, background: false });
    });
});
