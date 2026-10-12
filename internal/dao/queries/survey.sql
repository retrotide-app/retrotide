-- name: CreateSurvey :one
INSERT INTO board_surveys (id, org_id, board_id, title, template_key)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(title), sqlc.arg(template_key))
RETURNING *;

-- name: AddSurveyQuestion :one
INSERT INTO survey_questions (id, org_id, board_id, survey_id, position, kind, prompt, options, scale_min, scale_max)
VALUES (
    sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(survey_id), sqlc.arg(position), sqlc.arg(kind), sqlc.arg(prompt),
    sqlc.arg(options)::jsonb, sqlc.narg(scale_min), sqlc.narg(scale_max)
)
RETURNING *;

-- name: GetSurvey :one
SELECT * FROM board_surveys
WHERE board_surveys.board_id = sqlc.arg(board_id) AND board_surveys.id = sqlc.arg(id);

-- name: ListSurveys :many
SELECT * FROM board_surveys
WHERE board_surveys.board_id = $1
ORDER BY board_surveys.created_at, board_surveys.id;

-- name: CountSurveys :one
SELECT COUNT(*) FROM board_surveys WHERE board_surveys.board_id = $1;

-- name: ListSurveyQuestions :many
SELECT * FROM survey_questions
WHERE survey_questions.board_id = sqlc.arg(board_id) AND survey_questions.survey_id = sqlc.arg(survey_id)
ORDER BY survey_questions.position;

-- name: ListBoardSurveyQuestions :many
SELECT * FROM survey_questions
WHERE survey_questions.board_id = $1
ORDER BY survey_questions.survey_id, survey_questions.position;

-- name: CloseSurvey :one
UPDATE board_surveys SET status = 'closed', closed_at = COALESCE(board_surveys.closed_at, NOW())
WHERE board_surveys.board_id = sqlc.arg(board_id) AND board_surveys.id = sqlc.arg(id)
RETURNING *;

-- name: CloseOpenSurveys :execrows
UPDATE board_surveys SET status = 'closed', closed_at = NOW()
WHERE board_surveys.board_id = $1 AND board_surveys.status = 'open';

-- name: DeleteSurvey :execrows
DELETE FROM board_surveys
WHERE board_surveys.board_id = sqlc.arg(board_id) AND board_surveys.id = sqlc.arg(id);

-- name: AddSurveyResponse :exec
INSERT INTO survey_responses (org_id, board_id, survey_id, question_id, participant_id, answer)
VALUES (sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(survey_id), sqlc.arg(question_id), sqlc.arg(participant_id)::uuid, sqlc.arg(answer)::jsonb);

-- name: DeleteSurveyResponses :execrows
DELETE FROM survey_responses
WHERE survey_responses.board_id = sqlc.arg(board_id) AND survey_responses.survey_id = sqlc.arg(survey_id)
    AND survey_responses.participant_id = sqlc.arg(participant_id)::uuid;

-- name: ListSurveyResponses :many
SELECT survey_responses.id, survey_responses.survey_id, survey_responses.question_id, survey_responses.participant_id, survey_responses.answer
FROM survey_responses
WHERE survey_responses.board_id = $1
ORDER BY survey_responses.survey_id, survey_responses.question_id, survey_responses.id;

-- name: ListSurveyResponsesForSurvey :many
SELECT survey_responses.id, survey_responses.survey_id, survey_responses.question_id, survey_responses.participant_id, survey_responses.answer
FROM survey_responses
WHERE survey_responses.board_id = sqlc.arg(board_id) AND survey_responses.survey_id = sqlc.arg(survey_id)
ORDER BY survey_responses.question_id, survey_responses.id;

-- name: HandOverSurveyResponses :one
WITH moved AS (
    INSERT INTO survey_responses (org_id, board_id, survey_id, question_id, participant_id, answer)
    SELECT held.org_id, held.board_id, held.survey_id, held.question_id, sqlc.arg(to_participant_id)::uuid, held.answer
    FROM survey_responses AS held
    WHERE held.board_id = sqlc.arg(board_id) AND held.participant_id = sqlc.arg(from_participant_id)::uuid
    ON CONFLICT (question_id, participant_id) DO NOTHING
    RETURNING survey_responses.id
), dropped AS (
    DELETE FROM survey_responses
    WHERE survey_responses.board_id = sqlc.arg(board_id) AND survey_responses.participant_id = sqlc.arg(from_participant_id)::uuid
    RETURNING survey_responses.id
)
SELECT (SELECT COUNT(*) FROM moved)::bigint AS moved, (SELECT COUNT(*) FROM dropped)::bigint AS dropped;

-- name: SeverSurveyResponses :execrows
UPDATE survey_responses SET participant_id = NULL
WHERE survey_responses.board_id = $1 AND survey_responses.participant_id IS NOT NULL;
