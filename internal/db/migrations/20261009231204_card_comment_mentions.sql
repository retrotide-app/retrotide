-- +goose Up
CREATE TABLE card_mentions (
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    card_id UUID NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (board_id, card_id, user_id),
    CONSTRAINT card_mentions_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT card_mentions_card_fkey FOREIGN KEY (board_id, card_id) REFERENCES cards (board_id, id) ON DELETE CASCADE,
    CONSTRAINT card_mentions_member_fkey FOREIGN KEY (org_id, user_id) REFERENCES organisation_members (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX idx_card_mentions_user ON card_mentions (user_id, created_at DESC);
CREATE INDEX idx_card_mentions_member ON card_mentions (org_id, user_id);
CREATE INDEX idx_card_mentions_board ON card_mentions (org_id, board_id);

CREATE TABLE card_comment_mentions (
    org_id UUID NOT NULL,
    board_id UUID NOT NULL,
    comment_id UUID NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (board_id, comment_id, user_id),
    CONSTRAINT card_comment_mentions_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE,
    CONSTRAINT card_comment_mentions_comment_fkey FOREIGN KEY (board_id, comment_id) REFERENCES card_comments (board_id, id) ON DELETE CASCADE,
    CONSTRAINT card_comment_mentions_member_fkey FOREIGN KEY (org_id, user_id) REFERENCES organisation_members (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX idx_card_comment_mentions_user ON card_comment_mentions (user_id, created_at DESC);
CREATE INDEX idx_card_comment_mentions_member ON card_comment_mentions (org_id, user_id);
CREATE INDEX idx_card_comment_mentions_board ON card_comment_mentions (org_id, board_id);

-- +goose Down
DROP TABLE card_comment_mentions;
DROP TABLE card_mentions;
