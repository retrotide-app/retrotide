import { BOARD_COLOURS } from "../../domain/board/fields";
import { REACTIONS } from "../../domain/board/reactions";
import { BOARD_STAGES } from "../../domain/board/stages";
import type {
    BoardEvent,
    BoardEventsFrame,
    BoardJoinedPayload,
    BoardMePayload,
    BoardPresencePayload,
    BoardRefusalPayload,
    TeamActionItemsChangedPayload,
} from "../../types/api";
import type { ErrorPayload, PongPayload, RealtimeEvent, RealtimeEventName, RealtimeEventPayload } from "./events";

export type RealtimeEventGuard<K extends RealtimeEventName> = (data: unknown) => data is RealtimeEventPayload<K>;

type RealtimeGuardTable = Partial<{ [K in RealtimeEventName]: RealtimeEventGuard<K> }>;

type FieldCheck = (record: Record<string, unknown>) => boolean;

const BOARD_ROLES: ReadonlySet<unknown> = new Set(["facilitator", "participant"]);

const REFUSAL_REASONS: ReadonlySet<unknown> = new Set([
    "not_participant",
    "password_required",
    "account_required",
    "link_disabled",
    "removed",
    "archived",
]);

const VIEWER_STATES: ReadonlySet<unknown> = new Set(["active", "idle"]);

const COLOURS: ReadonlySet<unknown> = new Set(BOARD_COLOURS);

const SORT_MODES: ReadonlySet<unknown> = new Set(["manual", "votes"]);

const STAGES: ReadonlySet<unknown> = new Set(BOARD_STAGES);

const ACTION_ITEM_STATUSES: ReadonlySet<unknown> = new Set(["open", "done", "dropped"]);

const REVIEW_OUTCOMES: ReadonlySet<unknown> = new Set(["done", "carried", "dropped"]);

const DUE_DATE = /^\d{4}-\d{2}-\d{2}$/;

const REACTION_KEYS: ReadonlySet<unknown> = new Set(REACTIONS.map(option => option.key));

const SURVEY_STATUSES: ReadonlySet<unknown> = new Set(["open", "closed"]);

const QUESTION_KINDS: ReadonlySet<unknown> = new Set(["single_choice", "multiple_choice", "scale"]);

const UPLOADS_PREFIX = "/uploads/";

function asRecord(data: unknown): Record<string, unknown> | null {
    if (data === null || typeof data !== "object" || Array.isArray(data)) {
        return null;
    }

    return data as Record<string, unknown>;
}

function isFiniteNumber(value: unknown): boolean {
    return typeof value === "number" && Number.isFinite(value);
}

function isVersion(value: unknown): boolean {
    return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}

function isId(value: unknown): boolean {
    return typeof value === "string" && value.length > 0;
}

function isAbsentOrFiniteNumber(value: unknown): boolean {
    return value === undefined || isFiniteNumber(value);
}

function isAbsentOrString(value: unknown): boolean {
    return value === undefined || typeof value === "string";
}

function isRecordOf(value: unknown, check: FieldCheck): boolean {
    const record = asRecord(value);
    if (record === null) {
        return false;
    }

    return check(record);
}

function isListOf(value: unknown, check: (item: unknown) => boolean): boolean {
    return Array.isArray(value) && value.every(check);
}

function isCountMap(value: unknown): boolean {
    const record = asRecord(value);
    if (record === null) {
        return false;
    }

    return Object.values(record).every(count => typeof count === "number" && Number.isSafeInteger(count) && count >= 0);
}

function isAuthor(value: unknown): boolean {
    if (value === null) {
        return true;
    }

    return isRecordOf(value, author => isId(author.participant_id) && typeof author.display_name === "string");
}

function isCount(value: unknown): boolean {
    return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}

function isTotal(value: unknown): boolean {
    return value === null || isCount(value);
}

function isMine(value: unknown): boolean {
    return value === undefined || value === true;
}

function isAbsentOrPeople(value: unknown): boolean {
    return value === undefined || isListOf(value, isPersonView);
}

function isUploadPath(value: unknown): boolean {
    return typeof value === "string" && value.startsWith(UPLOADS_PREFIX);
}

function isReactionCount(value: unknown): boolean {
    return isRecordOf(value, count => REACTION_KEYS.has(count.reaction) && isCount(count.count));
}

function isCardView(value: unknown): boolean {
    return isRecordOf(
        value,
        card =>
            isId(card.id) &&
            isId(card.column_id) &&
            (card.parent_card_id === null || isId(card.parent_card_id)) &&
            typeof card.position === "string" &&
            typeof card.body === "string" &&
            (card.colour === null || COLOURS.has(card.colour)) &&
            isAuthor(card.author) &&
            isMine(card.is_mine) &&
            isAbsentOrPeople(card.mentions) &&
            typeof card.hidden === "boolean" &&
            isRecordOf(card.votes, votes => isTotal(votes.total)) &&
            isAbsentOrString(card.created_at) &&
            isAbsentOrString(card.updated_at) &&
            isMine(card.pinned) &&
            (card.image_path === undefined || isUploadPath(card.image_path)) &&
            (card.reactions === undefined || isListOf(card.reactions, isReactionCount)) &&
            (card.my_reactions === undefined || isListOf(card.my_reactions, key => REACTION_KEYS.has(key))),
    );
}

function isNullableCount(value: unknown): boolean {
    return value === null || isCount(value);
}

function isSurveyResults(value: unknown): boolean {
    if (value === null) {
        return true;
    }

    return isRecordOf(
        value,
        results =>
            isListOf(results.counts, isCount) &&
            isCount(results.responses) &&
            (results.average === null || isFiniteNumber(results.average)),
    );
}

function isSurveyQuestionView(value: unknown): boolean {
    return isRecordOf(
        value,
        question =>
            isId(question.id) &&
            QUESTION_KINDS.has(question.kind) &&
            typeof question.prompt === "string" &&
            isListOf(question.options, option => typeof option === "string") &&
            isNullableCount(question.scale_min) &&
            isNullableCount(question.scale_max) &&
            isSurveyResults(question.results),
    );
}

function isAnswerMap(value: unknown): boolean {
    const record = asRecord(value);
    if (record === null) {
        return false;
    }

    return Object.values(record).every(answer => isListOf(answer, isCount));
}

function isSurveyView(value: unknown): boolean {
    return isRecordOf(
        value,
        survey =>
            isId(survey.id) &&
            typeof survey.title === "string" &&
            typeof survey.template_key === "string" &&
            SURVEY_STATUSES.has(survey.status) &&
            isListOf(survey.questions, isSurveyQuestionView) &&
            isCount(survey.respondents) &&
            isAnswerMap(survey.my_answers),
    );
}

function isCommentView(value: unknown): boolean {
    return isRecordOf(
        value,
        comment =>
            isId(comment.id) &&
            isId(comment.card_id) &&
            typeof comment.body === "string" &&
            isAuthor(comment.author) &&
            isMine(comment.is_mine) &&
            isAbsentOrPeople(comment.mentions) &&
            isAbsentOrString(comment.created_at) &&
            isAbsentOrString(comment.updated_at),
    );
}

function isSettingsView(value: unknown): boolean {
    return isRecordOf(
        value,
        settings =>
            typeof settings.voting_open === "boolean" &&
            typeof settings.vote_counts_hidden === "boolean" &&
            isCount(settings.max_votes) &&
            isCount(settings.max_votes_per_card) &&
            typeof settings.comments_enabled === "boolean" &&
            typeof settings.merging_enabled === "boolean" &&
            SORT_MODES.has(settings.sort_mode) &&
            typeof settings.cards_locked === "boolean" &&
            typeof settings.moves_locked === "boolean" &&
            typeof settings.board_locked === "boolean" &&
            typeof settings.timer_play_sound === "boolean" &&
            typeof settings.timer_auto_lock === "boolean" &&
            typeof settings.sort_enforced === "boolean" &&
            typeof settings.anyone_can_edit === "boolean" &&
            typeof settings.show_card_dates === "boolean",
    );
}

function isNullableId(value: unknown): boolean {
    return value === null || isId(value);
}

function isSpotlightView(value: unknown): boolean {
    return isRecordOf(
        value,
        spotlight =>
            isNullableId(spotlight.presenting_card_id) &&
            isNullableId(spotlight.highlighted_card_id) &&
            isNullableId(spotlight.focused_column_id),
    );
}

function isTimestamp(value: unknown): boolean {
    return typeof value === "string" && !Number.isNaN(Date.parse(value));
}

function isTimerView(value: unknown): boolean {
    if (value === null) {
        return true;
    }

    return isRecordOf(value, timer => {
        if (!isCount(timer.duration_ms)) {
            return false;
        }

        const running = isTimestamp(timer.ends_at) && timer.paused_remaining_ms === null;
        const paused = timer.ends_at === null && isCount(timer.paused_remaining_ms);

        return running || paused;
    });
}

function isColumnView(value: unknown): boolean {
    return isRecordOf(
        value,
        column =>
            isId(column.id) &&
            typeof column.title === "string" &&
            typeof column.description === "string" &&
            COLOURS.has(column.colour) &&
            typeof column.position === "string" &&
            (column.max_votes === null || isCount(column.max_votes)) &&
            typeof column.cards_revealed === "boolean" &&
            typeof column.hidden === "boolean",
    );
}

function isParticipantView(value: unknown): boolean {
    return isRecordOf(
        value,
        participant =>
            isId(participant.participant_id) &&
            typeof participant.display_name === "string" &&
            BOARD_ROLES.has(participant.role),
    );
}

function isPositionView(value: unknown): boolean {
    return isRecordOf(value, placed => isId(placed.id) && typeof placed.position === "string");
}

function isPersonView(value: unknown): boolean {
    return isRecordOf(value, person => isId(person.user_id) && typeof person.display_name === "string");
}

function isDueDate(value: unknown): boolean {
    return value === null || (typeof value === "string" && DUE_DATE.test(value));
}

function hasActionItemFields(item: Record<string, unknown>): boolean {
    return (
        isId(item.id) &&
        typeof item.title === "string" &&
        (item.assignee === null || isPersonView(item.assignee)) &&
        isListOf(item.mentions, isPersonView) &&
        isDueDate(item.due_on) &&
        ACTION_ITEM_STATUSES.has(item.status) &&
        (item.source_card_id === null || isId(item.source_card_id))
    );
}

function isActionItemView(value: unknown): boolean {
    return isRecordOf(value, item => hasActionItemFields(item) && isMine(item.is_mine));
}

function isPreviousActionView(value: unknown): boolean {
    return isRecordOf(
        value,
        item => hasActionItemFields(item) && (item.review === null || REVIEW_OUTCOMES.has(item.review)),
    );
}

const BOARD_EVENT_FIELDS: Record<BoardEvent["type"], FieldCheck> = {
    resync: () => true,
    board_updated: event => typeof event.title === "string" && typeof event.context === "string",
    participant_upserted: event => isParticipantView(event.participant),
    column_upserted: event => isColumnView(event.column),
    column_moved: event =>
        isId(event.column_id) && typeof event.position === "string" && isListOf(event.rewritten, isPositionView),
    column_deleted: event => isId(event.column_id),
    column_cleared: event => isId(event.column_id),
    card_upserted: event => isCardView(event.card),
    card_moved: event =>
        isId(event.card_id) &&
        isId(event.column_id) &&
        typeof event.position === "string" &&
        isListOf(event.rewritten, isPositionView),
    card_deleted: event => isId(event.card_id),
    cards_merged: event =>
        isId(event.card_id) &&
        isId(event.parent_card_id) &&
        isId(event.column_id) &&
        typeof event.position === "string" &&
        isListOf(event.restacked, isId),
    card_unmerged: event =>
        isId(event.card_id) &&
        isId(event.column_id) &&
        typeof event.position === "string" &&
        isListOf(event.rewritten, isPositionView),
    hidden_counts: event => isCountMap(event.hidden_counts),
    settings_changed: event => isSettingsView(event.settings),
    votes_changed: event => isId(event.card_id) && isTotal(event.total),
    votes_reset: () => true,
    comment_upserted: event => isCommentView(event.comment),
    comment_deleted: event => isId(event.comment_id) && isId(event.card_id),
    participant_removed: event => isId(event.participant_id),
    cards_revealed: event =>
        STAGES.has(event.stage) && isListOf(event.cards, isCardView) && isListOf(event.comments, isCommentView),
    cards_hidden: event => isCountMap(event.hidden_counts),
    stage_changed: event => STAGES.has(event.stage),
    timer_changed: event => isTimerView(event.timer) && isTimestamp(event.server_now),
    action_item_upserted: event => isActionItemView(event.action_item),
    action_item_removed: event => isId(event.action_item_id),
    previous_action_upserted: event => isPreviousActionView(event.previous_action),
    column_revealed: event =>
        isId(event.column_id) && isListOf(event.cards, isCardView) && isListOf(event.comments, isCommentView),
    column_hidden: event => isId(event.column_id),
    column_votes_reset: event => isId(event.column_id),
    spotlight_changed: event => isSpotlightView(event.spotlight),
    card_reactions_changed: event =>
        isId(event.card_id) &&
        isListOf(event.reactions, isReactionCount) &&
        isListOf(event.my_reactions, key => REACTION_KEYS.has(key)),
    card_pinned: event => isId(event.card_id) && typeof event.pinned === "boolean",
    card_image_changed: event => isId(event.card_id) && (event.image_path === null || isUploadPath(event.image_path)),
    board_background_changed: event => event.background_path === null || isUploadPath(event.background_path),
    survey_upserted: event => isSurveyView(event.survey),
    survey_removed: event => isId(event.survey_id),
};

function isBoardEventType(value: unknown): value is BoardEvent["type"] {
    return typeof value === "string" && Object.hasOwn(BOARD_EVENT_FIELDS, value);
}

export function isBoardEvent(data: unknown): data is BoardEvent {
    const record = asRecord(data);
    if (record === null || !isVersion(record.version) || !isBoardEventType(record.type)) {
        return false;
    }

    return BOARD_EVENT_FIELDS[record.type](record);
}

function eventsWithinFrame(events: unknown[], from: number, to: number): boolean {
    let previous = from - 1;

    for (const event of events) {
        if (!isBoardEvent(event) || event.version <= previous || event.version > to) {
            return false;
        }

        previous = event.version;
    }

    return true;
}

export function isPongPayload(data: unknown): data is PongPayload {
    const record = asRecord(data);
    if (record === null) {
        return false;
    }

    return isAbsentOrFiniteNumber(record.nonce) && isAbsentOrString(record.server_now);
}

export function isErrorPayload(data: unknown): data is ErrorPayload {
    return isRecordOf(data, record => typeof record.error === "string");
}

export function isBoardJoinedPayload(data: unknown): data is BoardJoinedPayload {
    return isRecordOf(
        data,
        record =>
            isId(record.board_id) &&
            isVersion(record.version) &&
            (record.participant_id === null || isId(record.participant_id)) &&
            BOARD_ROLES.has(record.role),
    );
}

export function isBoardRefusalPayload(data: unknown): data is BoardRefusalPayload {
    return isRecordOf(data, record => isId(record.board_id) && REFUSAL_REASONS.has(record.reason));
}

export function isBoardPresencePayload(data: unknown): data is BoardPresencePayload {
    return isRecordOf(
        data,
        record =>
            isId(record.board_id) &&
            isListOf(record.viewers, viewer =>
                isRecordOf(
                    viewer,
                    entry =>
                        isId(entry.participant_id) &&
                        typeof entry.display_name === "string" &&
                        (entry.state === undefined || VIEWER_STATES.has(entry.state)),
                ),
            ),
    );
}

export function isBoardEventsFrame(data: unknown): data is BoardEventsFrame {
    const record = asRecord(data);
    if (record === null || !isId(record.board_id) || !isVersion(record.from) || !isVersion(record.to)) {
        return false;
    }

    const from = record.from as number;
    const to = record.to as number;

    if (from > to || !Array.isArray(record.events)) {
        return false;
    }

    return eventsWithinFrame(record.events, from, to);
}

export function isBoardMePayload(data: unknown): data is BoardMePayload {
    return isRecordOf(
        data,
        record =>
            isId(record.board_id) &&
            isVersion(record.version) &&
            isFiniteNumber(record.votes_remaining) &&
            isCountMap(record.my_votes),
    );
}

export function isTeamActionItemsChangedPayload(data: unknown): data is TeamActionItemsChangedPayload {
    return isRecordOf(data, record => isId(record.team_id));
}

export const REALTIME_EVENT_GUARDS = {
    pong: isPongPayload,
    error: isErrorPayload,
    board_joined: isBoardJoinedPayload,
    board_join_rejected: isBoardRefusalPayload,
    board_access_revoked: isBoardRefusalPayload,
    board_presence: isBoardPresencePayload,
    board_events: isBoardEventsFrame,
    board_me: isBoardMePayload,
    team_action_items_changed: isTeamActionItemsChangedPayload,
} as const satisfies RealtimeGuardTable;

export type GuardedRealtimeEventName = keyof typeof REALTIME_EVENT_GUARDS;

const GUARDED_EVENT_NAMES: ReadonlySet<string> = new Set<string>(Object.keys(REALTIME_EVENT_GUARDS));

export function passesRealtimeGuard(event: RealtimeEvent): boolean {
    if (!GUARDED_EVENT_NAMES.has(event.type)) {
        return true;
    }

    const guard: (data: unknown) => boolean = REALTIME_EVENT_GUARDS[event.type as GuardedRealtimeEventName];

    return guard(event.data);
}
