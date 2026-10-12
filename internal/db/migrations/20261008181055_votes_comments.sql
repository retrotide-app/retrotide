-- +goose Up

CREATE TABLE card_votes (
    board_id UUID NOT NULL,
    card_id UUID NOT NULL,
    participant_id UUID NOT NULL,
    count SMALLINT NOT NULL,
    PRIMARY KEY (card_id, participant_id),
    CONSTRAINT card_votes_count_check CHECK (count > 0),
    CONSTRAINT card_votes_card_fkey FOREIGN KEY (board_id, card_id) REFERENCES cards (board_id, id) ON DELETE CASCADE,
    CONSTRAINT card_votes_participant_fkey FOREIGN KEY (board_id, participant_id) REFERENCES board_participants (board_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_card_votes_participant ON card_votes (board_id, participant_id);
CREATE INDEX idx_card_votes_board_card ON card_votes (board_id, card_id);

CREATE TABLE board_vote_budgets (
    board_id UUID NOT NULL,
    participant_id UUID NOT NULL,
    used SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (board_id, participant_id),
    CONSTRAINT board_vote_budgets_used_check CHECK (used >= 0),
    CONSTRAINT board_vote_budgets_participant_fkey FOREIGN KEY (board_id, participant_id) REFERENCES board_participants (board_id, id) ON DELETE CASCADE
);

CREATE TABLE card_comments (
    id UUID PRIMARY KEY,
    board_id UUID NOT NULL,
    card_id UUID NOT NULL,
    author_participant_id UUID,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT card_comments_board_id_id_key UNIQUE (board_id, id),
    CONSTRAINT card_comments_card_fkey FOREIGN KEY (board_id, card_id) REFERENCES cards (board_id, id) ON DELETE CASCADE,
    CONSTRAINT card_comments_author_fkey FOREIGN KEY (board_id, author_participant_id) REFERENCES board_participants (board_id, id) ON DELETE SET NULL (author_participant_id),
    CONSTRAINT card_comments_body_length CHECK (char_length(body) BETWEEN 1 AND 1000)
);
CREATE INDEX idx_card_comments_card ON card_comments (board_id, card_id, created_at);
CREATE INDEX idx_card_comments_author ON card_comments (board_id, author_participant_id) WHERE author_participant_id IS NOT NULL;

-- +goose Down

DROP TABLE card_comments;
DROP TABLE board_vote_budgets;
DROP TABLE card_votes;
