-- name: GetUserTOTP :one
SELECT * FROM user_totp WHERE user_id = $1;

-- name: BeginUserTOTP :one
INSERT INTO user_totp (user_id, secret)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE
SET secret = EXCLUDED.secret, last_used_step = 0, created_at = NOW()
WHERE user_totp.confirmed_at IS NULL
RETURNING *;

-- name: ConfirmUserTOTP :one
UPDATE user_totp
SET confirmed_at = NOW(), last_used_step = $2
WHERE user_id = $1 AND confirmed_at IS NULL AND last_used_step < $2
RETURNING *;

-- name: UseUserTOTPStep :execrows
UPDATE user_totp
SET last_used_step = $2
WHERE user_id = $1 AND confirmed_at IS NOT NULL AND last_used_step < $2;

-- name: DeleteUserTOTP :execrows
DELETE FROM user_totp WHERE user_id = $1;

-- name: CreateTwoFactorRecoveryCode :exec
INSERT INTO two_factor_recovery_codes (user_id, code_hash)
VALUES ($1, $2);

-- name: UseTwoFactorRecoveryCode :execrows
UPDATE two_factor_recovery_codes
SET used_at = NOW()
WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL;

-- name: CreateTwoFactorChallenge :exec
INSERT INTO two_factor_challenges (token_hash, user_id, return_to, remember, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetLiveTwoFactorChallenge :one
SELECT * FROM two_factor_challenges
WHERE token_hash = $1 AND expires_at > NOW();

-- name: AttemptTwoFactorChallenge :one
UPDATE two_factor_challenges
SET attempts = attempts + 1
WHERE token_hash = $1 AND expires_at > NOW() AND attempts < $2
RETURNING *;

-- name: ConsumeTwoFactorChallenge :one
DELETE FROM two_factor_challenges
WHERE token_hash = $1 AND expires_at > NOW()
RETURNING *;

-- name: DeleteExpiredTwoFactorChallenges :execrows
DELETE FROM two_factor_challenges WHERE expires_at <= NOW();

-- name: CreateTwoFactorTrustedDevice :exec
INSERT INTO two_factor_trusted_devices (token_hash, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: TwoFactorTrustedDeviceLive :one
SELECT EXISTS (
    SELECT 1 FROM two_factor_trusted_devices
    WHERE token_hash = $1 AND user_id = $2 AND expires_at > NOW()
);

-- name: DeleteTwoFactorTrustedDevicesForUser :exec
DELETE FROM two_factor_trusted_devices WHERE user_id = $1;

-- name: DeleteExpiredTwoFactorTrustedDevices :execrows
DELETE FROM two_factor_trusted_devices WHERE expires_at <= NOW();
