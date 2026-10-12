-- +goose Up
ALTER TABLE user_identities DROP CONSTRAINT user_identities_provider_check;
ALTER TABLE user_identities ADD CONSTRAINT user_identities_provider_check CHECK (provider IN ('google', 'microsoft'));
ALTER TABLE oidc_flows ADD COLUMN then_link TEXT;
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_provider_check;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_provider_check CHECK (provider IN ('google', 'microsoft'));
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_purpose_check;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_purpose_check CHECK (purpose IN ('sign_in', 'link', 'reauthenticate'));
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_link_has_user;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_link_has_user CHECK ((purpose IN ('link', 'reauthenticate')) = (user_id IS NOT NULL));
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_link_has_session;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_link_has_session CHECK ((purpose IN ('link', 'reauthenticate')) = (session_token_hash IS NOT NULL));
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_then_link_check CHECK ((purpose = 'reauthenticate') = (then_link IS NOT NULL) AND (then_link IS NULL OR (then_link IN ('google', 'microsoft') AND then_link <> provider)));

-- +goose Down
DELETE FROM oidc_flows WHERE provider <> 'google' OR purpose = 'reauthenticate';
DELETE FROM user_identities WHERE provider <> 'google';
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_then_link_check;
ALTER TABLE oidc_flows DROP COLUMN then_link;
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_link_has_session;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_link_has_session CHECK ((purpose = 'link') = (session_token_hash IS NOT NULL));
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_link_has_user;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_link_has_user CHECK ((purpose = 'link') = (user_id IS NOT NULL));
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_purpose_check;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_purpose_check CHECK (purpose IN ('sign_in', 'link'));
ALTER TABLE oidc_flows DROP CONSTRAINT oidc_flows_provider_check;
ALTER TABLE oidc_flows ADD CONSTRAINT oidc_flows_provider_check CHECK (provider IN ('google'));
ALTER TABLE user_identities DROP CONSTRAINT user_identities_provider_check;
ALTER TABLE user_identities ADD CONSTRAINT user_identities_provider_check CHECK (provider IN ('google'));
