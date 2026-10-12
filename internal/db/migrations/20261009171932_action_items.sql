-- +goose Up
CREATE TABLE action_items (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    team_id UUID,
    board_id UUID,
    source_card_id UUID,
    title TEXT NOT NULL,
    assignee_user_id UUID,
    due_on DATE,
    status TEXT NOT NULL DEFAULT 'open',
    completed_at TIMESTAMPTZ,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT action_items_title_length CHECK (char_length(title) BETWEEN 1 AND 500),
    CONSTRAINT action_items_status_check CHECK (status IN ('open', 'done', 'dropped')),
    CONSTRAINT action_items_done_has_completed_at CHECK ((status = 'done') = (completed_at IS NOT NULL)),
    CONSTRAINT action_items_org_id_id_key UNIQUE (org_id, id),
    CONSTRAINT action_items_source_card_key UNIQUE (source_card_id),
    CONSTRAINT action_items_org_fkey FOREIGN KEY (org_id) REFERENCES organisations (id) ON DELETE CASCADE,
    CONSTRAINT action_items_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE,
    CONSTRAINT action_items_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE SET NULL (board_id),
    CONSTRAINT action_items_card_fkey FOREIGN KEY (board_id, source_card_id) REFERENCES cards (board_id, id) ON DELETE SET NULL (source_card_id),
    CONSTRAINT action_items_assignee_fkey FOREIGN KEY (org_id, assignee_user_id) REFERENCES organisation_members (org_id, user_id) ON DELETE SET NULL (assignee_user_id),
    CONSTRAINT action_items_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL
);
CREATE INDEX idx_action_items_team_status ON action_items (org_id, team_id, status) WHERE team_id IS NOT NULL;
CREATE INDEX idx_action_items_board ON action_items (org_id, board_id) WHERE board_id IS NOT NULL;
CREATE INDEX idx_action_items_assignee ON action_items (org_id, assignee_user_id) WHERE assignee_user_id IS NOT NULL;
CREATE INDEX idx_action_items_org_created ON action_items (org_id, created_at DESC);
CREATE INDEX idx_action_items_created_by ON action_items (created_by) WHERE created_by IS NOT NULL;

CREATE TABLE action_item_reviews (
    org_id UUID NOT NULL,
    action_item_id UUID NOT NULL,
    board_id UUID NOT NULL,
    outcome TEXT NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (action_item_id, board_id),
    CONSTRAINT action_item_reviews_outcome_check CHECK (outcome IN ('done', 'carried', 'dropped')),
    CONSTRAINT action_item_reviews_item_fkey FOREIGN KEY (org_id, action_item_id) REFERENCES action_items (org_id, id) ON DELETE CASCADE,
    CONSTRAINT action_item_reviews_board_fkey FOREIGN KEY (org_id, board_id) REFERENCES boards (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_action_item_reviews_board ON action_item_reviews (org_id, board_id);

CREATE TABLE action_item_mentions (
    org_id UUID NOT NULL,
    action_item_id UUID NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (action_item_id, user_id),
    CONSTRAINT action_item_mentions_item_fkey FOREIGN KEY (org_id, action_item_id) REFERENCES action_items (org_id, id) ON DELETE CASCADE,
    CONSTRAINT action_item_mentions_member_fkey FOREIGN KEY (org_id, user_id) REFERENCES organisation_members (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX idx_action_item_mentions_user ON action_item_mentions (user_id, created_at DESC);
CREATE INDEX idx_action_item_mentions_member ON action_item_mentions (org_id, user_id);

-- +goose Down
DROP TABLE action_item_mentions;
DROP TABLE action_item_reviews;
DROP TABLE action_items;
