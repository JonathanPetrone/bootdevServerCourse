-- name: CreateChirp :one
INSERT INTO chirps(created_at, updated_at, body, user_id)
VALUES (
    NOW(),
    NOW(),
    @body,
    @user_id
)
RETURNING *;