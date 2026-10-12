import type {
    ActionItemListItem,
    ActionItemListResponse,
    ActionItemMentionView,
    ActionItemView,
    CardMentionView,
    CommentMentionView,
    PersonView,
    PreviousActionView,
} from "../../types/api";

export const ACTION_ITEM_ID = "00000000-0000-4000-8000-0000000000a9";

export function makePerson(overrides: Partial<PersonView> = {}): PersonView {
    return {
        user_id: "00000000-0000-4000-8000-000000000001",
        display_name: "Sam",
        ...overrides,
    };
}

export function makeActionItem(overrides: Partial<ActionItemView> = {}): ActionItemView {
    return {
        id: ACTION_ITEM_ID,
        title: "Book the retro room",
        assignee: null,
        mentions: [],
        due_on: null,
        status: "open",
        source_card_id: null,
        ...overrides,
    };
}

export function makePreviousAction(overrides: Partial<PreviousActionView> = {}): PreviousActionView {
    return {
        ...makeActionItem({ id: "00000000-0000-4000-8000-0000000000a8", title: "Write the release notes" }),
        review: null,
        ...overrides,
    };
}

export function makeActionItemListItem(overrides: Partial<ActionItemListItem> = {}): ActionItemListItem {
    return {
        ...makeActionItem(),
        board: { id: "00000000-0000-4000-8000-0000000000b1", title: "Sprint 12" },
        team_id: "00000000-0000-4000-8000-0000000000d1",
        team_name: "Crew",
        ...overrides,
    };
}

export function makeActionItemList(overrides: Partial<ActionItemListResponse> = {}): ActionItemListResponse {
    return {
        action_items: [makeActionItemListItem()],
        total: 1,
        limit: 50,
        offset: 0,
        counts: { open: 1, done: 0, dropped: 0 },
        completion_rate: 0,
        ...overrides,
    };
}

export function makeMention(overrides: Partial<ActionItemMentionView> = {}): ActionItemMentionView {
    return {
        kind: "action_item",
        action_item_id: ACTION_ITEM_ID,
        title: "Ask @Sam about the release",
        status: "open",
        due_on: null,
        org_id: "00000000-0000-4000-8000-0000000000c1",
        org_slug: "harbour",
        org_name: "Harbour",
        team_id: "00000000-0000-4000-8000-0000000000d1",
        board_id: "00000000-0000-4000-8000-0000000000b1",
        board_title: "Sprint 12",
        ...overrides,
    };
}

export function makeCardMention(overrides: Partial<CardMentionView> = {}): CardMentionView {
    return {
        kind: "card",
        card_id: "00000000-0000-4000-8000-0000000000e1",
        body: "Thanks @Sam for the release",
        org_id: "00000000-0000-4000-8000-0000000000c1",
        org_slug: "harbour",
        org_name: "Harbour",
        team_id: "00000000-0000-4000-8000-0000000000d1",
        board_id: "00000000-0000-4000-8000-0000000000b1",
        board_title: "Sprint 12",
        ...overrides,
    };
}

export function makeCommentMention(overrides: Partial<CommentMentionView> = {}): CommentMentionView {
    return {
        kind: "comment",
        card_id: "00000000-0000-4000-8000-0000000000e1",
        comment_id: "00000000-0000-4000-8000-0000000000f1",
        body: "Agreed, @Sam can pair on it",
        org_id: "00000000-0000-4000-8000-0000000000c1",
        org_slug: "harbour",
        org_name: "Harbour",
        team_id: "00000000-0000-4000-8000-0000000000d1",
        board_id: "00000000-0000-4000-8000-0000000000b1",
        board_title: "Sprint 12",
        ...overrides,
    };
}
