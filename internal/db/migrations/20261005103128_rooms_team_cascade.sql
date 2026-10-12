-- +goose Up
ALTER TABLE rooms DROP CONSTRAINT rooms_team_fkey;
ALTER TABLE rooms ADD CONSTRAINT rooms_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE rooms DROP CONSTRAINT rooms_team_fkey;
ALTER TABLE rooms ADD CONSTRAINT rooms_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id);
