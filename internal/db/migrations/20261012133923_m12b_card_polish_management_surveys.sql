-- +goose Up
CREATE TABLE media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    kind TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'image/webp',
    width INT NOT NULL,
    height INT NOT NULL,
    byte_size INT NOT NULL,
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT media_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT media_board_id_id_key UNIQUE (board_id, id),
    CONSTRAINT media_kind_check CHECK (kind IN ('card', 'background')),
    CONSTRAINT media_content_type_check CHECK (content_type = 'image/webp'),
    CONSTRAINT media_dimensions_check CHECK (width BETWEEN 1 AND 4096 AND height BETWEEN 1 AND 4096),
    CONSTRAINT media_byte_size_check CHECK (byte_size > 0 AND byte_size = octet_length(data))
);
CREATE INDEX idx_media_created_at ON media (created_at);

ALTER TABLE cards
    ADD COLUMN pinned_at TIMESTAMPTZ,
    ADD COLUMN image_media_id UUID,
    ADD CONSTRAINT cards_image_fkey FOREIGN KEY (board_id, image_media_id) REFERENCES media (board_id, id) ON DELETE SET NULL (image_media_id);
CREATE INDEX idx_cards_image_media ON cards (board_id, image_media_id) WHERE image_media_id IS NOT NULL;

ALTER TABLE boards
    ADD COLUMN show_card_dates BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN background_media_id UUID,
    ADD CONSTRAINT boards_background_fkey FOREIGN KEY (id, background_media_id) REFERENCES media (board_id, id) ON DELETE SET NULL (background_media_id);

CREATE TABLE card_reactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    card_id UUID NOT NULL,
    participant_id UUID,
    reaction TEXT NOT NULL,
    CONSTRAINT card_reactions_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT card_reactions_card_fkey FOREIGN KEY (board_id, card_id) REFERENCES cards (board_id, id) ON DELETE CASCADE,
    CONSTRAINT card_reactions_participant_fkey FOREIGN KEY (board_id, participant_id) REFERENCES board_participants (board_id, id) ON DELETE SET NULL (participant_id),
    CONSTRAINT card_reactions_reaction_check CHECK (reaction IN ('thumbs_up', 'thumbs_down', 'heart', 'laugh', 'wow', 'sad', 'angry', 'party', 'rocket', 'fire', 'clap', 'thinking', 'eyes', 'hundred', 'star')),
    CONSTRAINT card_reactions_once UNIQUE (card_id, participant_id, reaction)
);
CREATE INDEX idx_card_reactions_board_card ON card_reactions (board_id, card_id);

CREATE TABLE board_pins (
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, board_id),
    CONSTRAINT board_pins_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT board_pins_member_fkey FOREIGN KEY (org_id, user_id) REFERENCES organisation_members (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX idx_board_pins_org_user ON board_pins (org_id, user_id);

CREATE TABLE board_surveys (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    title TEXT NOT NULL,
    template_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    CONSTRAINT board_surveys_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT board_surveys_board_id_id_key UNIQUE (board_id, id),
    CONSTRAINT board_surveys_title_length CHECK (char_length(title) BETWEEN 1 AND 120),
    CONSTRAINT board_surveys_status_check CHECK (status IN ('open', 'closed')),
    CONSTRAINT board_surveys_closed_check CHECK ((status = 'closed') = (closed_at IS NOT NULL))
);
CREATE INDEX idx_board_surveys_board ON board_surveys (board_id, created_at);

CREATE TABLE survey_questions (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    survey_id UUID NOT NULL,
    position SMALLINT NOT NULL,
    kind TEXT NOT NULL,
    prompt TEXT NOT NULL,
    options JSONB NOT NULL DEFAULT '[]'::jsonb,
    scale_min SMALLINT,
    scale_max SMALLINT,
    CONSTRAINT survey_questions_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT survey_questions_survey_fkey FOREIGN KEY (board_id, survey_id) REFERENCES board_surveys (board_id, id) ON DELETE CASCADE,
    CONSTRAINT survey_questions_board_id_id_key UNIQUE (board_id, id),
    CONSTRAINT survey_questions_position_key UNIQUE (survey_id, position),
    CONSTRAINT survey_questions_kind_check CHECK (kind IN ('single_choice', 'multiple_choice', 'scale')),
    CONSTRAINT survey_questions_prompt_length CHECK (char_length(prompt) BETWEEN 1 AND 300),
    CONSTRAINT survey_questions_options_array CHECK (jsonb_typeof(options) = 'array'),
    CONSTRAINT survey_questions_shape_check CHECK (
        (kind = 'scale' AND scale_min IS NOT NULL AND scale_max IS NOT NULL AND scale_min >= 0 AND scale_max <= 10 AND scale_min < scale_max AND jsonb_array_length(options) = 0)
        OR (kind <> 'scale' AND scale_min IS NULL AND scale_max IS NULL AND jsonb_array_length(options) BETWEEN 2 AND 10)
    )
);

CREATE TABLE survey_responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    survey_id UUID NOT NULL,
    question_id UUID NOT NULL,
    participant_id UUID,
    answer JSONB NOT NULL,
    CONSTRAINT survey_responses_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT survey_responses_survey_fkey FOREIGN KEY (board_id, survey_id) REFERENCES board_surveys (board_id, id) ON DELETE CASCADE,
    CONSTRAINT survey_responses_question_fkey FOREIGN KEY (board_id, question_id) REFERENCES survey_questions (board_id, id) ON DELETE CASCADE,
    CONSTRAINT survey_responses_participant_fkey FOREIGN KEY (board_id, participant_id) REFERENCES board_participants (board_id, id) ON DELETE SET NULL (participant_id),
    CONSTRAINT survey_responses_once UNIQUE (question_id, participant_id),
    CONSTRAINT survey_responses_answer_array CHECK (jsonb_typeof(answer) = 'array' AND jsonb_array_length(answer) BETWEEN 1 AND 10)
);
CREATE INDEX idx_survey_responses_survey ON survey_responses (board_id, survey_id);

-- +goose Down
DROP TABLE survey_responses;

DROP TABLE survey_questions;

DROP TABLE board_surveys;

DROP TABLE board_pins;

DROP TABLE card_reactions;

ALTER TABLE boards
    DROP CONSTRAINT boards_background_fkey,
    DROP COLUMN background_media_id,
    DROP COLUMN show_card_dates;

DROP INDEX idx_cards_image_media;

ALTER TABLE cards
    DROP CONSTRAINT cards_image_fkey,
    DROP COLUMN image_media_id,
    DROP COLUMN pinned_at;

DROP TABLE media;
