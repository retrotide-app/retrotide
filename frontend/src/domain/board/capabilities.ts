import type { ActionItemView, BoardSnapshot, BoardView, CardView, CommentView } from "../../types/api";

export interface BoardActions {
    addCards: boolean;
    moveCards: boolean;
    manageColumns: boolean;
    removeAnyCard: boolean;
    describeBoard: boolean;
    removeColumns: boolean;
    controlTimer: boolean;
    spotlight: boolean;
}

export interface CardActions {
    edit: boolean;
    recolour: boolean;
    remove: boolean;
    move: boolean;
}

export function isArchived(board: BoardView): boolean {
    return board.archived_at !== undefined && board.archived_at !== "";
}

export function isReadOnly(board: BoardView): boolean {
    return isArchived(board) || board.closed;
}

function facilitates({ board, viewer }: BoardSnapshot): boolean {
    return !isReadOnly(board) && viewer.capabilities.includes("board.facilitate");
}

function participates({ board, viewer }: BoardSnapshot): boolean {
    return !isReadOnly(board) && viewer.capabilities.includes("board.participate");
}

function seated(snapshot: BoardSnapshot): boolean {
    return participates(snapshot) && snapshot.viewer.participant_id !== null;
}

function writesCards(snapshot: BoardSnapshot): boolean {
    return !snapshot.settings.cards_locked || facilitates(snapshot);
}

function movesCards(snapshot: BoardSnapshot): boolean {
    return facilitates(snapshot) || (participates(snapshot) && !snapshot.settings.moves_locked);
}

function writing({ board }: BoardSnapshot): boolean {
    return board.stage === "write";
}

function discussing({ board }: BoardSnapshot): boolean {
    return board.stage === "discuss";
}

function edits(snapshot: BoardSnapshot): boolean {
    return facilitates(snapshot) || (snapshot.settings.anyone_can_edit && seated(snapshot));
}

export function columnHidden(snapshot: BoardSnapshot, columnId: string): boolean {
    return snapshot.columns.some(column => column.id === columnId && column.hidden);
}

export function columnCardsHidden(snapshot: BoardSnapshot, columnId: string): boolean {
    const column = snapshot.columns.find(held => held.id === columnId);

    return snapshot.board.cards_hidden && column?.cards_revealed !== true;
}

export function boardActions(snapshot: BoardSnapshot): BoardActions {
    const { board, viewer } = snapshot;

    return {
        addCards: seated(snapshot) && writesCards(snapshot) && writing(snapshot),
        moveCards: movesCards(snapshot) && writing(snapshot),
        manageColumns: edits(snapshot),
        removeAnyCard: facilitates(snapshot),
        describeBoard: !isReadOnly(board) && viewer.capabilities.includes("board.manage"),
        removeColumns: facilitates(snapshot),
        controlTimer: edits(snapshot),
        spotlight: facilitates(snapshot),
    };
}

export function canExportBoard({ viewer }: BoardSnapshot): boolean {
    return viewer.capabilities.includes("board.export");
}

export interface BoardInteractions {
    vote: boolean;
    comment: boolean;
    mergeCards: boolean;
    facilitate: boolean;
}

export interface CardInteractions {
    vote: boolean;
    comment: boolean;
    merge: boolean;
    unmerge: boolean;
}

export interface CommentActions {
    edit: boolean;
    remove: boolean;
}

export const NO_INTERACTIONS: BoardInteractions = { vote: false, comment: false, mergeCards: false, facilitate: false };

export const NO_CARD_INTERACTIONS: CardInteractions = { vote: false, comment: false, merge: false, unmerge: false };

export const NO_COMMENT_ACTIONS: CommentActions = { edit: false, remove: false };

export function boardInteractions(snapshot: BoardSnapshot): BoardInteractions {
    const { board, settings } = snapshot;

    return {
        vote: seated(snapshot) && settings.voting_open && board.stage === "vote",
        comment: seated(snapshot) && settings.comments_enabled && discussing(snapshot),
        mergeCards: movesCards(snapshot) && settings.merging_enabled && !board.cards_hidden && discussing(snapshot),
        facilitate: facilitates(snapshot),
    };
}

export function cardInteractions(snapshot: BoardSnapshot, card: CardView): CardInteractions {
    if (columnHidden(snapshot, card.column_id)) {
        return NO_CARD_INTERACTIONS;
    }

    const interactions = boardInteractions(snapshot);
    const stacked = card.parent_card_id !== null && snapshot.cards.some(held => held.id === card.parent_card_id);

    return {
        vote: interactions.vote,
        comment: interactions.comment,
        merge: interactions.mergeCards && !stacked,
        unmerge: movesCards(snapshot) && stacked && !snapshot.board.cards_hidden && discussing(snapshot),
    };
}

export function commentActions(snapshot: BoardSnapshot, comment: CommentView): CommentActions {
    const own = seated(snapshot) && comment.is_mine === true && discussing(snapshot);

    return {
        edit: own && snapshot.settings.comments_enabled,
        remove: own || facilitates(snapshot),
    };
}

export interface ActionItemActions {
    add: boolean;
    edit: boolean;
    mark: boolean;
    remove: boolean;
    removeOwn: boolean;
    name: boolean;
    review: boolean;
}

export const NO_ACTION_ITEM_ACTIONS: ActionItemActions = {
    add: false,
    edit: false,
    mark: false,
    remove: false,
    removeOwn: false,
    name: false,
    review: false,
};

function inActionsStage({ board }: BoardSnapshot): boolean {
    return board.stage === "actions";
}

export function actionItemActions(snapshot: BoardSnapshot, member: boolean): ActionItemActions {
    const { board } = snapshot;
    const writes = seated(snapshot) && inActionsStage(snapshot);
    const markedAfterEnd = member && board.closed && !isArchived(board);

    return {
        add: writes,
        edit: writes,
        mark: (participates(snapshot) && inActionsStage(snapshot)) || markedAfterEnd,
        remove: facilitates(snapshot),
        removeOwn: writes,
        name: writes && member,
        review: member && participates(snapshot) && board.team_id !== undefined,
    };
}

export function canRemoveActionItem(actions: ActionItemActions, item: ActionItemView): boolean {
    return actions.remove || (actions.removeOwn && item.is_mine === true);
}

export function cardActions(snapshot: BoardSnapshot, card: CardView): CardActions {
    if (columnHidden(snapshot, card.column_id)) {
        return { edit: false, recolour: false, remove: false, move: false };
    }

    const mine = card.is_mine === true;
    const authors = participates(snapshot) && mine && writesCards(snapshot) && writing(snapshot);
    const moves = movesCards(snapshot) && writing(snapshot) && (mine || facilitates(snapshot));

    return {
        edit: authors,
        recolour: authors,
        remove: authors || facilitates(snapshot),
        move: moves,
    };
}

export interface CardPolishActions {
    react: boolean;
    pin: boolean;
    setImage: boolean;
}

export interface SurveyActions {
    manage: boolean;
    respond: boolean;
}

export interface BoardManagementActions {
    archive: boolean;
    remove: boolean;
    move: boolean;
    importCards: boolean;
    background: boolean;
}

function organisationMember({ viewer }: BoardSnapshot): boolean {
    return viewer.capabilities.includes("board.export");
}

function manages({ viewer }: BoardSnapshot): boolean {
    return viewer.capabilities.includes("board.manage");
}

export function cardPolishActions(snapshot: BoardSnapshot, card: CardView): CardPolishActions {
    const seenByAll = !card.hidden && !columnHidden(snapshot, card.column_id);

    return {
        react: seated(snapshot) && seenByAll,
        pin: facilitates(snapshot) && seenByAll,
        setImage: cardActions(snapshot, card).edit && organisationMember(snapshot),
    };
}

export function surveyActions(snapshot: BoardSnapshot): SurveyActions {
    return {
        manage: facilitates(snapshot),
        respond: seated(snapshot),
    };
}

export function boardManagementActions(snapshot: BoardSnapshot): BoardManagementActions {
    return {
        archive: manages(snapshot),
        remove: manages(snapshot),
        move: manages(snapshot),
        importCards: seated(snapshot) && facilitates(snapshot) && writing(snapshot),
        background: !isReadOnly(snapshot.board) && manages(snapshot) && organisationMember(snapshot),
    };
}
