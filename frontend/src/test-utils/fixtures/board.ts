import type {
    BoardCapability,
    BoardCommandResponse,
    BoardListItem,
    BoardSettingsView,
    BoardSnapshot,
    BoardView,
    CardView,
    ColumnView,
    CommentView,
    MeView,
    OrgBoardListItem,
    ParticipantView,
    SpotlightView,
    SurveyQuestionView,
    SurveyView,
    TemplateResponse,
    TimerView,
    ViewerView,
} from "../../types/api";

export const BOARD_ID = "00000000-0000-4000-8000-0000000000b1";

export const VIEWER_PARTICIPANT_ID = "00000000-0000-4000-8000-0000000000f1";

export const FACILITATOR_CAPABILITIES: readonly BoardCapability[] = [
    "board.view",
    "board.participate",
    "board.facilitate",
    "board.manage",
    "board.export",
];

export const PARTICIPANT_CAPABILITIES: readonly BoardCapability[] = ["board.view", "board.participate", "board.export"];

export function makeBoardView(overrides: Partial<BoardView> = {}): BoardView {
    return {
        id: BOARD_ID,
        team_id: "00000000-0000-4000-8000-0000000000d1",
        title: "Sprint 12",
        context: "",
        template_key: "start-stop-continue",
        anonymity: "named",
        cards_hidden: true,
        stage: "write",
        closed: false,
        show_prime_directive: true,
        created_at: "2026-10-01T09:00:00Z",
        ...overrides,
    };
}

export function makeBoardViewer(overrides: Partial<ViewerView> = {}): ViewerView {
    return {
        participant_id: VIEWER_PARTICIPANT_ID,
        role: "facilitator",
        capabilities: [...FACILITATOR_CAPABILITIES],
        ...overrides,
    };
}

export function makeParticipantViewer(overrides: Partial<ViewerView> = {}): ViewerView {
    return makeBoardViewer({ role: "participant", capabilities: [...PARTICIPANT_CAPABILITIES], ...overrides });
}

export function makeColumn(overrides: Partial<ColumnView> = {}): ColumnView {
    return {
        id: "00000000-0000-4000-8000-000000000c01",
        title: "Start",
        description: "What should we begin doing?",
        colour: "green",
        position: "a0",
        max_votes: null,
        cards_revealed: false,
        hidden: false,
        ...overrides,
    };
}

export function makeSpotlight(overrides: Partial<SpotlightView> = {}): SpotlightView {
    return {
        presenting_card_id: null,
        highlighted_card_id: null,
        focused_column_id: null,
        ...overrides,
    };
}

export function makeCard(overrides: Partial<CardView> = {}): CardView {
    return {
        id: "00000000-0000-4000-8000-000000000a01",
        column_id: "00000000-0000-4000-8000-000000000c01",
        parent_card_id: null,
        position: "a0",
        body: "Pair on reviews",
        colour: null,
        author: { participant_id: VIEWER_PARTICIPANT_ID, display_name: "Sam" },
        is_mine: true,
        hidden: true,
        votes: { total: 0 },
        created_at: "2026-10-01T09:05:00Z",
        updated_at: "2026-10-01T09:05:00Z",
        ...overrides,
    };
}

export function makeBoardSettings(overrides: Partial<BoardSettingsView> = {}): BoardSettingsView {
    return {
        voting_open: true,
        vote_counts_hidden: false,
        max_votes: 6,
        max_votes_per_card: 0,
        comments_enabled: true,
        merging_enabled: true,
        sort_mode: "manual",
        cards_locked: false,
        moves_locked: false,
        board_locked: false,
        timer_play_sound: true,
        timer_auto_lock: false,
        sort_enforced: false,
        anyone_can_edit: false,
        show_card_dates: false,
        ...overrides,
    };
}

export function makeMe(overrides: Partial<MeView> = {}): MeView {
    return {
        votes_remaining: 6,
        my_votes: {},
        ...overrides,
    };
}

export function makeComment(overrides: Partial<CommentView> = {}): CommentView {
    return {
        id: "00000000-0000-4000-8000-000000000e01",
        card_id: "00000000-0000-4000-8000-000000000a01",
        body: "Agreed, reviews drag on",
        author: { participant_id: VIEWER_PARTICIPANT_ID, display_name: "Sam" },
        is_mine: true,
        created_at: "2026-10-01T09:10:00Z",
        ...overrides,
    };
}

export function makeBoardParticipant(overrides: Partial<ParticipantView> = {}): ParticipantView {
    return {
        participant_id: VIEWER_PARTICIPANT_ID,
        display_name: "Sam",
        role: "facilitator",
        ...overrides,
    };
}

export function makeBoardSnapshot(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return {
        board_id: BOARD_ID,
        version: 3,
        board: makeBoardView(),
        settings: makeBoardSettings(),
        viewer: makeBoardViewer(),
        me: makeMe(),
        columns: [
            makeColumn(),
            makeColumn({
                id: "00000000-0000-4000-8000-000000000c02",
                title: "Stop",
                description: "What should we stop doing?",
                colour: "red",
                position: "a1",
            }),
            makeColumn({
                id: "00000000-0000-4000-8000-000000000c03",
                title: "Continue",
                description: "What should we keep doing?",
                colour: "blue",
                position: "a2",
            }),
        ],
        cards: [makeCard()],
        comments: [],
        hidden_counts: { "00000000-0000-4000-8000-000000000c02": 2 },
        participants: [makeBoardParticipant()],
        timer: null,
        action_items: [],
        previous_actions: [],
        spotlight: makeSpotlight(),
        surveys: [],
        server_now: "2026-10-01T09:30:00.000Z",
        ...overrides,
    };
}

export function makeSurveyQuestion(overrides: Partial<SurveyQuestionView> = {}): SurveyQuestionView {
    return {
        id: "00000000-0000-4000-8000-000000000b51",
        kind: "scale",
        prompt: "How sustainable was the pace?",
        options: [],
        scale_min: 1,
        scale_max: 5,
        results: null,
        ...overrides,
    };
}

export function makeSurvey(overrides: Partial<SurveyView> = {}): SurveyView {
    return {
        id: "00000000-0000-4000-8000-000000000b50",
        title: "Sprint health",
        template_key: "",
        status: "open",
        questions: [makeSurveyQuestion()],
        respondents: 0,
        my_answers: {},
        ...overrides,
    };
}

export function makeTimer(overrides: Partial<TimerView> = {}): TimerView {
    return {
        duration_ms: 300_000,
        ends_at: "2026-10-01T09:35:00.000Z",
        paused_remaining_ms: null,
        ...overrides,
    };
}

export function makeBoardListItem(overrides: Partial<BoardListItem> = {}): BoardListItem {
    return {
        id: BOARD_ID,
        team_id: "00000000-0000-4000-8000-0000000000d1",
        title: "Sprint 12",
        created_at: "2026-10-01T09:00:00Z",
        ...overrides,
    };
}

export function makeOrgBoardListItem(overrides: Partial<OrgBoardListItem> = {}): OrgBoardListItem {
    return {
        ...makeBoardListItem(),
        team_name: "Crew",
        template_key: "start-stop-continue",
        ...overrides,
    };
}

export function makeBoardTemplate(overrides: Partial<TemplateResponse> = {}): TemplateResponse {
    return {
        key: "blank",
        name: "Blank",
        description: "Start with one column, then rename it and add the columns you need.",
        columns: [{ title: "Notes", description: "Anything worth raising.", colour: "grey" }],
        ...overrides,
    };
}

export function makeBoardCommandResponse(overrides: Partial<BoardCommandResponse> = {}): BoardCommandResponse {
    return {
        version: 4,
        ...overrides,
    };
}
